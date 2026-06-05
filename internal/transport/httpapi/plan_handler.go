package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/service/plansvc"
)

type bandwidthRequest struct {
	RateLimitRx        string `json:"rate_limit_rx"`
	RateLimitTx        string `json:"rate_limit_tx"`
	BurstRx            string `json:"burst_rx"`
	BurstTx            string `json:"burst_tx"`
	BurstThresholdRx   string `json:"burst_threshold_rx"`
	BurstThresholdTx   string `json:"burst_threshold_tx"`
	BurstTime          string `json:"burst_time"`
	Priority           int32  `json:"priority"`
	MikrotikRateString string `json:"mikrotik_rate_string"`
}

type planRequest struct {
	Name         string           `json:"name" validate:"required,min=1,max=120"`
	ServiceType  string           `json:"service_type" validate:"required,oneof=pppoe hotspot dhcp"`
	PriceIDR     int64            `json:"price_idr" validate:"gte=0"`
	TaxBps       int32            `json:"tax_bps" validate:"gte=0,lte=100000"`
	BillingCycle string           `json:"billing_cycle" validate:"required,oneof=monthly fixed profile prepaid_topup"`
	ActiveDays   int32            `json:"active_days" validate:"gt=0"`
	DataQuotaMB  *int64           `json:"data_quota_mb"`
	TimeQuotaSec *int64           `json:"time_quota_sec"`
	IsUnlimited  bool             `json:"is_unlimited"`
	PoolName     string           `json:"pool_name"`
	IsolirPlanID *int64           `json:"isolir_plan_id"`
	Bandwidth    bandwidthRequest `json:"bandwidth"`
}

func (req planRequest) toInput() plansvc.Input {
	return plansvc.Input{
		Name:         req.Name,
		ServiceType:  plan.ServiceType(req.ServiceType),
		PriceIDR:     req.PriceIDR,
		TaxBps:       req.TaxBps,
		BillingCycle: plan.BillingCycle(req.BillingCycle),
		ActiveDays:   req.ActiveDays,
		DataQuotaMB:  req.DataQuotaMB,
		TimeQuotaSec: req.TimeQuotaSec,
		IsUnlimited:  req.IsUnlimited,
		PoolName:     req.PoolName,
		IsolirPlanID: req.IsolirPlanID,
		Bandwidth: plansvc.BandwidthInput{
			RateLimitRx:        req.Bandwidth.RateLimitRx,
			RateLimitTx:        req.Bandwidth.RateLimitTx,
			BurstRx:            req.Bandwidth.BurstRx,
			BurstTx:            req.Bandwidth.BurstTx,
			BurstThresholdRx:   req.Bandwidth.BurstThresholdRx,
			BurstThresholdTx:   req.Bandwidth.BurstThresholdTx,
			BurstTime:          req.Bandwidth.BurstTime,
			Priority:           req.Bandwidth.Priority,
			MikrotikRateString: req.Bandwidth.MikrotikRateString,
		},
	}
}

type planResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ServiceType  string `json:"service_type"`
	PriceIDR     int64  `json:"price_idr"`
	TaxBps       int32  `json:"tax_bps"`
	TotalIDR     int64  `json:"total_idr"`
	BillingCycle string `json:"billing_cycle"`
	ActiveDays   int32  `json:"active_days"`
	IsUnlimited  bool   `json:"is_unlimited"`
	PoolName     string `json:"pool_name"`
	GroupName    string `json:"group_name"`
	MikrotikRate string `json:"mikrotik_rate"`
}

func toPlanResponse(res plansvc.Result) planResponse {
	return planResponse{
		ID:           res.Plan.ID,
		Name:         res.Plan.Name,
		ServiceType:  string(res.Plan.ServiceType),
		PriceIDR:     res.Plan.PriceIDR,
		TaxBps:       res.Plan.TaxBps,
		TotalIDR:     res.Plan.TotalIDR(),
		BillingCycle: string(res.Plan.BillingCycle),
		ActiveDays:   res.Plan.ActiveDays,
		IsUnlimited:  res.Plan.IsUnlimited,
		PoolName:     res.Plan.PoolName,
		GroupName:    res.Plan.GroupName(),
		MikrotikRate: res.Bandwidth.MikrotikRate(),
	}
}

func (a *API) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req planRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	res, err := a.plans.Create(r.Context(), TenantID(r.Context()), UserID(r.Context()), req.toInput())
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPlanResponse(res))
}

func (a *API) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	var req planRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	res, err := a.plans.Update(r.Context(), TenantID(r.Context()), UserID(r.Context()), id, req.toInput())
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(res))
}

func (a *API) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	res, err := a.plans.Get(r.Context(), TenantID(r.Context()), id)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(res))
}

func (a *API) handleListPlans(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	plans, total, err := a.plans.List(r.Context(), TenantID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]planResponse, len(plans))
	for i, p := range plans {
		items[i] = toPlanResponse(plansvc.Result{Plan: p})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (a *API) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.plans.Delete(r.Context(), TenantID(r.Context()), UserID(r.Context()), id); err != nil {
		writeError(w, a.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
