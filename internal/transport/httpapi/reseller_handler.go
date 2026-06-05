package httpapi

import (
	"net/http"
	"strconv"
)

type topupRequest struct {
	UserID    int64  `json:"user_id" validate:"required,gt=0"`
	AmountIDR int64  `json:"amount_idr" validate:"required,gt=0"`
	Method    string `json:"method" validate:"omitempty,oneof=cash transfer gateway balance"`
}

func (a *API) handleTopup(w http.ResponseWriter, r *http.Request) {
	var req topupRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	method := req.Method
	if method == "" {
		method = "transfer"
	}
	dep, balance, err := a.reseller.Topup(r.Context(), TenantID(r.Context()), req.UserID, req.AmountIDR, method)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"deposit_id": dep.ID, "amount_idr": dep.AmountIDR, "balance_idr": balance,
	})
}

func (a *API) handleListDeposits(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	limit, offset := pagination(r)
	deposits, err := a.reseller.ListDeposits(r.Context(), TenantID(r.Context()), userID, limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]map[string]any, len(deposits))
	for i, d := range deposits {
		items[i] = map[string]any{"id": d.ID, "amount_idr": d.AmountIDR, "method": d.Method, "status": d.Status, "created_at": d.CreatedAt}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *API) handleListCommissions(w http.ResponseWriter, r *http.Request) {
	resellerID, _ := strconv.ParseInt(r.URL.Query().Get("reseller_id"), 10, 64)
	limit, offset := pagination(r)
	commissions, err := a.reseller.ListCommissions(r.Context(), TenantID(r.Context()), resellerID, limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]map[string]any, len(commissions))
	for i, c := range commissions {
		items[i] = map[string]any{"id": c.ID, "amount_idr": c.AmountIDR, "source_payment_id": c.SourcePaymentID, "status": c.Status, "created_at": c.CreatedAt}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}
