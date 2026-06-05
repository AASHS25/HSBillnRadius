package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/tenant"
	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	"github.com/aashs25/hsbillnradius/internal/service/acssvc"
)

// errorBody is the JSON envelope for error responses.
type errorBody struct {
	Error  string            `json:"error"`
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

// badRequestError marks a client error (malformed body) mapped to HTTP 400.
type badRequestError struct{ msg string }

func (e badRequestError) Error() string { return e.msg }

// errBadRequest builds a 400-class error with a safe, client-facing message.
func errBadRequest(msg string) error { return badRequestError{msg: msg} }

// writeJSON serializes v with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError maps an error to a safe HTTP response. Unknown errors become a
// generic 500 and the real error is logged, never leaked to the client.
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	// Validation errors carry per-field detail.
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{
			Error:  "validation failed",
			Code:   "validation_failed",
			Fields: fieldErrors(valErrs),
		})
		return
	}

	// Malformed-request errors are safe to echo back verbatim.
	var badReq badRequestError
	if errors.As(err, &badReq) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: badReq.msg, Code: "bad_request"})
		return
	}

	status, code := classify(err)
	if status == http.StatusInternalServerError {
		log.Error("request failed", slog.Any("error", err))
		writeJSON(w, status, errorBody{Error: "internal server error", Code: "internal_error"})
		return
	}
	writeJSON(w, status, errorBody{Error: err.Error(), Code: code})
}

// classify maps a domain error to an HTTP status and stable error code.
func classify(err error) (int, string) {
	switch {
	case errors.Is(err, iam.ErrInvalidCredential), errors.Is(err, token.ErrInvalidToken),
		errors.Is(err, customer.ErrPortalInvalid):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, iam.ErrUserInactive):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, iam.ErrUserNotFound), errors.Is(err, tenant.ErrNotFound),
		errors.Is(err, iam.ErrRoleNotFound), errors.Is(err, plan.ErrNotFound),
		errors.Is(err, customer.ErrNotFound), errors.Is(err, billing.ErrInvoiceNotFound),
		errors.Is(err, billing.ErrPaymentNotFound), errors.Is(err, billing.ErrGatewayNotEnabled),
		errors.Is(err, ticket.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, iam.ErrEmailTaken), errors.Is(err, tenant.ErrSlugTaken),
		errors.Is(err, plan.ErrNameTaken), errors.Is(err, customer.ErrNoTaken),
		errors.Is(err, customer.ErrUsernameTaken), errors.Is(err, billing.ErrInvoiceExists),
		errors.Is(err, billing.ErrAlreadyPaid), errors.Is(err, billing.ErrPaymentNotUnique):
		return http.StatusConflict, "conflict"
	case errors.Is(err, billing.ErrInvalidStatus), errors.Is(err, ticket.ErrInvalidStatus),
		errors.Is(err, ticket.ErrInvalidTicket):
		return http.StatusUnprocessableEntity, "invalid_status"
	case errors.Is(err, iam.ErrWeakPassword), errors.Is(err, iam.ErrInvalidEmail),
		errors.Is(err, tenant.ErrInvalidSlug), errors.Is(err, plan.ErrInvalidPlan),
		errors.Is(err, plan.ErrInvalidPrice), errors.Is(err, customer.ErrInvalidCust),
		errors.Is(err, customer.ErrPlanRequired), errors.Is(err, customer.ErrCredsRequired):
		return http.StatusUnprocessableEntity, "invalid_input"
	case errors.Is(err, acssvc.ErrNotConfigured):
		return http.StatusServiceUnavailable, "acs_unavailable"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// fieldErrors turns validator errors into a field->message map.
func fieldErrors(errs validator.ValidationErrors) map[string]string {
	out := make(map[string]string, len(errs))
	for _, e := range errs {
		out[e.Field()] = "failed rule: " + e.Tag()
	}
	return out
}
