package httpapi

import (
	"net/http"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/service/customersvc"
)

const dateLayout = "2006-01-02"

type customerRequest struct {
	CustomerNo    string   `json:"customer_no" validate:"required,max=64"`
	Name          string   `json:"name" validate:"required,max=160"`
	IDCardNo      string   `json:"id_card_no" validate:"max=64"`
	Email         string   `json:"email" validate:"omitempty,email"`
	PhoneWA       string   `json:"phone_wa" validate:"max=32"`
	Address       string   `json:"address" validate:"max=500"`
	Lat           *float64 `json:"lat"`
	Lng           *float64 `json:"lng"`
	InstallDate   string   `json:"install_date" validate:"omitempty,datetime=2006-01-02"`
	Status        string   `json:"status" validate:"omitempty,oneof=new active isolated suspended terminated free"`
	PlanID        *int64   `json:"plan_id"`
	ResellerID    *int64   `json:"reseller_id"`
	PppoeUsername string   `json:"pppoe_username" validate:"max=64"`
	PppoePassword string   `json:"pppoe_password" validate:"max=128"`
	Notes         string   `json:"notes" validate:"max=1000"`
}

func (req customerRequest) toInput() customersvc.Input {
	var installDate *time.Time
	if req.InstallDate != "" {
		if d, err := time.Parse(dateLayout, req.InstallDate); err == nil {
			installDate = &d
		}
	}
	return customersvc.Input{
		CustomerNo:    req.CustomerNo,
		Name:          req.Name,
		IDCardNo:      req.IDCardNo,
		Email:         req.Email,
		PhoneWA:       req.PhoneWA,
		Address:       req.Address,
		Lat:           req.Lat,
		Lng:           req.Lng,
		InstallDate:   installDate,
		Status:        customer.Status(req.Status),
		PlanID:        req.PlanID,
		ResellerID:    req.ResellerID,
		PppoeUsername: req.PppoeUsername,
		PppoePassword: req.PppoePassword,
		Notes:         req.Notes,
	}
}

type customerResponse struct {
	ID            int64    `json:"id"`
	CustomerNo    string   `json:"customer_no"`
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	PhoneWA       string   `json:"phone_wa"`
	Address       string   `json:"address"`
	Lat           *float64 `json:"lat"`
	Lng           *float64 `json:"lng"`
	Status        string   `json:"status"`
	PlanID        *int64   `json:"plan_id"`
	PppoeUsername string   `json:"pppoe_username"`
	BalanceIDR    int64    `json:"balance_idr"`
}

func toCustomerResponse(c customer.Customer) customerResponse {
	return customerResponse{
		ID:            c.ID,
		CustomerNo:    c.CustomerNo,
		Name:          c.Name,
		Email:         c.Email,
		PhoneWA:       c.PhoneWA,
		Address:       c.Address,
		Lat:           c.Lat,
		Lng:           c.Lng,
		Status:        string(c.Status),
		PlanID:        c.PlanID,
		PppoeUsername: c.PppoeUsername,
		BalanceIDR:    c.BalanceIDR,
	}
}

func (a *API) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	c, err := a.customers.Create(r.Context(), TenantID(r.Context()), UserID(r.Context()), req.toInput())
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCustomerResponse(c))
}

func (a *API) handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	var req customerRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	c, err := a.customers.Update(r.Context(), TenantID(r.Context()), UserID(r.Context()), id, req.toInput())
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toCustomerResponse(c))
}

func (a *API) handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	c, err := a.customers.Get(r.Context(), TenantID(r.Context()), id)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toCustomerResponse(c))
}

func (a *API) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	customers, total, err := a.customers.List(r.Context(), TenantID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]customerResponse, len(customers))
	for i, c := range customers {
		items[i] = toCustomerResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (a *API) handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.customers.Delete(r.Context(), TenantID(r.Context()), UserID(r.Context()), id); err != nil {
		writeError(w, a.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
