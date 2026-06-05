package httpapi

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/ports/payment"
	"github.com/aashs25/hsbillnradius/internal/service/paymentsvc"
)

const maxWebhookBytes = 1 << 20

type configureGatewayRequest struct {
	Provider     string            `json:"provider" validate:"required,oneof=midtrans xendit duitku tripay"`
	Config       map[string]string `json:"config" validate:"required"`
	IsActive     bool              `json:"is_active"`
	IsProduction bool              `json:"is_production"`
}

type createChargeRequest struct {
	InvoiceID int64  `json:"invoice_id" validate:"required,gt=0"`
	Provider  string `json:"provider" validate:"required,oneof=midtrans xendit"`
}

func (a *API) handleConfigurePaymentGateway(w http.ResponseWriter, r *http.Request) {
	var req configureGatewayRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	cfg, err := a.payments.ConfigureGateway(r.Context(), billing.GatewayConfig{
		TenantID:     TenantID(r.Context()),
		Provider:     req.Provider,
		Config:       req.Config,
		IsActive:     req.IsActive,
		IsProduction: req.IsProduction,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": cfg.ID, "provider": cfg.Provider})
}

func (a *API) handleCreateCharge(w http.ResponseWriter, r *http.Request) {
	var req createChargeRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	resp, err := a.payments.CreateCharge(r.Context(), TenantID(r.Context()), req.InvoiceID, payment.Provider(req.Provider))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ref": resp.Ref, "redirect_url": resp.RedirectURL, "status": string(resp.Status),
	})
}

// handlePaymentWebhook is a public endpoint (verified by provider signature, not
// JWT). It always returns 200 for already-processed or accepted callbacks so the
// provider stops retrying; only signature failures and unknown payments error.
func (a *API) handlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	provider := payment.Provider(chi.URLParam(r, "provider"))
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, a.log, errBadRequest("cannot read body"))
		return
	}

	if err := a.payments.HandleCallback(r.Context(), provider, raw, r.Header); err != nil {
		if paymentsvc.IsUnverified(err) {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid signature", Code: "unauthorized"})
			return
		}
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
