package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	"github.com/aashs25/hsbillnradius/internal/service/ticketsvc"
)

type portalLoginRequest struct {
	TenantSlug string `json:"tenant_slug" validate:"required"`
	CustomerNo string `json:"customer_no" validate:"required"`
	Password   string `json:"password" validate:"required"`
}

type portalTicketRequest struct {
	Type        string `json:"type" validate:"required,oneof=trouble install other"`
	Subject     string `json:"subject" validate:"required,max=200"`
	Description string `json:"description" validate:"max=4000"`
}

type setPortalPasswordRequest struct {
	Password string `json:"password" validate:"required,min=8,max=128"`
}

func (a *API) handlePortalLogin(w http.ResponseWriter, r *http.Request) {
	var req portalLoginRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	cust, err := a.customers.PortalLogin(r.Context(), req.TenantSlug, req.CustomerNo, req.Password)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	accessToken, expiresAt, err := a.tokens.IssueAccess(token.AccessInput{
		TenantID:    cust.TenantID,
		CustomerID:  cust.ID,
		Permissions: []string{"portal"},
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_at":   expiresAt,
		"customer":     map[string]any{"id": cust.ID, "name": cust.Name, "customer_no": cust.CustomerNo},
	})
}

func (a *API) handlePortalMe(w http.ResponseWriter, r *http.Request) {
	c, err := a.customers.Get(r.Context(), TenantID(r.Context()), CustomerID(r.Context()))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toCustomerResponse(c))
}

func (a *API) handlePortalInvoices(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	invoices, err := a.billing.ListByCustomer(r.Context(), TenantID(r.Context()), CustomerID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]invoiceResponse, len(invoices))
	for i, inv := range invoices {
		items[i] = toInvoiceResponse(inv)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *API) handlePortalTickets(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	tickets, err := a.tickets.ListByCustomer(r.Context(), TenantID(r.Context()), CustomerID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]map[string]any, len(tickets))
	for i, t := range tickets {
		items[i] = ticketDTO(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *API) handlePortalCreateTicket(w http.ResponseWriter, r *http.Request) {
	var req portalTicketRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	cid := CustomerID(r.Context())
	t, err := a.tickets.Create(r.Context(), TenantID(r.Context()), 0, ticketsvc.CreateInput{
		CustomerID:  &cid,
		Type:        ticket.Type(req.Type),
		Subject:     req.Subject,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticketDTO(t))
}

// handleSetPortalPassword is an admin action to set a customer's portal password.
func (a *API) handleSetPortalPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	var req setPortalPasswordRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.customers.SetPortalPassword(r.Context(), TenantID(r.Context()), id, req.Password); err != nil {
		writeError(w, a.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
