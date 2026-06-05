package httpapi

import (
	"net/http"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
)

type generateInvoiceRequest struct {
	CustomerID  int64  `json:"customer_id" validate:"required,gt=0"`
	PeriodStart string `json:"period_start" validate:"omitempty,datetime=2006-01-02"`
}

type payInvoiceRequest struct {
	Method string `json:"method" validate:"required,oneof=cash transfer balance gateway"`
}

type invoiceResponse struct {
	ID          int64            `json:"id"`
	InvoiceNo   string           `json:"invoice_no"`
	CustomerID  int64            `json:"customer_id"`
	PeriodStart string           `json:"period_start"`
	PeriodEnd   string           `json:"period_end"`
	DueDate     string           `json:"due_date"`
	SubtotalIDR int64            `json:"subtotal_idr"`
	TaxIDR      int64            `json:"tax_idr"`
	DiscountIDR int64            `json:"discount_idr"`
	TotalIDR    int64            `json:"total_idr"`
	Status      string           `json:"status"`
	Type        string           `json:"type"`
	Items       []invoiceItemDTO `json:"items,omitempty"`
}

type invoiceItemDTO struct {
	Description  string `json:"description"`
	Qty          int32  `json:"qty"`
	UnitPriceIDR int64  `json:"unit_price_idr"`
	AmountIDR    int64  `json:"amount_idr"`
	Type         string `json:"type"`
}

func toInvoiceResponse(inv billing.Invoice) invoiceResponse {
	const layout = "2006-01-02"
	resp := invoiceResponse{
		ID:          inv.ID,
		InvoiceNo:   inv.InvoiceNo,
		CustomerID:  inv.CustomerID,
		PeriodStart: inv.PeriodStart.Format(layout),
		PeriodEnd:   inv.PeriodEnd.Format(layout),
		DueDate:     inv.DueDate.Format(layout),
		SubtotalIDR: inv.SubtotalIDR,
		TaxIDR:      inv.TaxIDR,
		DiscountIDR: inv.DiscountIDR,
		TotalIDR:    inv.TotalIDR,
		Status:      string(inv.Status),
		Type:        string(inv.Type),
	}
	for _, it := range inv.Items {
		resp.Items = append(resp.Items, invoiceItemDTO{
			Description: it.Description, Qty: it.Qty, UnitPriceIDR: it.UnitPriceIDR,
			AmountIDR: it.AmountIDR, Type: string(it.Type),
		})
	}
	return resp
}

func (a *API) handleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	var req generateInvoiceRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	periodStart := time.Now()
	if req.PeriodStart != "" {
		if d, err := time.Parse("2006-01-02", req.PeriodStart); err == nil {
			periodStart = d
		}
	}
	inv, err := a.billing.GenerateMonthly(r.Context(), TenantID(r.Context()), req.CustomerID, UserID(r.Context()), periodStart)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toInvoiceResponse(inv))
}

func (a *API) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	invoices, err := a.billing.List(r.Context(), TenantID(r.Context()), limit, offset)
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

func (a *API) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	inv, err := a.billing.Get(r.Context(), TenantID(r.Context()), id)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toInvoiceResponse(inv))
}

func (a *API) handlePayInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	var req payInvoiceRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	pay, err := a.billing.PayInvoice(r.Context(), TenantID(r.Context()), id, UserID(r.Context()), billing.PaymentMethod(req.Method))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payment_id": pay.ID, "amount_idr": pay.AmountIDR, "status": string(pay.Status),
	})
}

func (a *API) handleVoidInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.billing.Void(r.Context(), TenantID(r.Context()), UserID(r.Context()), id); err != nil {
		writeError(w, a.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleReportSummary(w http.ResponseWriter, r *http.Request) {
	const layout = "2006-01-02"
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now.AddDate(0, 0, 1)
	if v := r.URL.Query().Get("from"); v != "" {
		if d, err := time.Parse(layout, v); err == nil {
			from = d
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if d, err := time.Parse(layout, v); err == nil {
			to = d
		}
	}
	summary, err := a.billing.Report(r.Context(), TenantID(r.Context()), from, to)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
