package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/voucher"
	"github.com/aashs25/hsbillnradius/internal/service/vouchersvc"
)

type generateBatchRequest struct {
	PlanID   int64  `json:"plan_id" validate:"required,gt=0"`
	Prefix   string `json:"prefix" validate:"max=16"`
	Qty      int32  `json:"qty" validate:"required,gt=0,lte=5000"`
	PriceIDR int64  `json:"price_idr" validate:"gte=0"`
}

type voucherDTO struct {
	Code     string `json:"code"`
	Username string `json:"username"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

func toVoucherDTO(v voucher.Voucher) voucherDTO {
	return voucherDTO{Code: v.Code, Username: v.Username, Password: v.Password, Status: string(v.Status)}
}

func (a *API) handleGenerateVouchers(w http.ResponseWriter, r *http.Request) {
	var req generateBatchRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	batch, vouchers, err := a.vouchers.GenerateBatch(r.Context(), TenantID(r.Context()), UserID(r.Context()), vouchersvc.GenerateInput{
		PlanID: req.PlanID, Prefix: req.Prefix, Qty: req.Qty, PriceIDR: req.PriceIDR,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]voucherDTO, len(vouchers))
	for i, v := range vouchers {
		items[i] = toVoucherDTO(v)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batch.ID, "qty": batch.Qty, "vouchers": items})
}

func (a *API) handleListVoucherBatches(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	batches, err := a.vouchers.ListBatches(r.Context(), TenantID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	type batchDTO struct {
		ID       int64 `json:"id"`
		PlanID   int64 `json:"plan_id"`
		Qty      int32 `json:"qty"`
		PriceIDR int64 `json:"price_idr"`
	}
	items := make([]batchDTO, len(batches))
	for i, b := range batches {
		items[i] = batchDTO{ID: b.ID, PlanID: b.PlanID, Qty: b.Qty, PriceIDR: b.PriceIDR}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *API) handleListBatchVouchers(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	limit, offset := pagination(r)
	vouchers, err := a.vouchers.ListVouchers(r.Context(), TenantID(r.Context()), id, limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]voucherDTO, len(vouchers))
	for i, v := range vouchers {
		items[i] = toVoucherDTO(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}
