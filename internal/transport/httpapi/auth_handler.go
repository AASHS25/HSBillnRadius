package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/service/authsvc"
)

func (a *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}

	res, err := a.auth.Register(r.Context(), authsvc.RegisterInput{
		TenantName: req.TenantName,
		TenantSlug: req.TenantSlug,
		Name:       req.Name,
		Email:      req.Email,
		Password:   req.Password,
		UserAgent:  r.UserAgent(),
		IP:         clientIP(r),
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAuthResponse(res))
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}

	res, err := a.auth.Login(r.Context(), authsvc.LoginInput{
		TenantSlug: req.TenantSlug,
		Email:      req.Email,
		Password:   req.Password,
		UserAgent:  r.UserAgent(),
		IP:         clientIP(r),
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(res))
}

func (a *API) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}

	res, err := a.auth.Refresh(r.Context(), authsvc.RefreshInput{
		RefreshToken: req.RefreshToken,
		UserAgent:    r.UserAgent(),
		IP:           clientIP(r),
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(res))
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, a.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user, tnt, perms, err := a.auth.Profile(r.Context(), UserID(r.Context()))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		User:        toUserResponse(user),
		Tenant:      toTenantResponse(tnt),
		Permissions: perms,
	})
}

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	tenantID := TenantID(r.Context())

	users, total, err := a.auth.ListUsers(r.Context(), tenantID, limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}

	items := make([]userResponse, len(users))
	for i, u := range users {
		items[i] = toUserResponse(u)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (a *API) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	tenantID := TenantID(r.Context())

	entries, err := a.auth.ListAuditLogs(r.Context(), tenantID, limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}

	type auditItem struct {
		ID        int64  `json:"id"`
		Action    string `json:"action"`
		Entity    string `json:"entity"`
		EntityID  string `json:"entity_id"`
		ActorID   *int64 `json:"actor_user_id"`
		CreatedAt string `json:"created_at"`
	}
	items := make([]auditItem, len(entries))
	for i, e := range entries {
		items[i] = auditItem{
			ID:        e.ID,
			Action:    e.Action,
			Entity:    e.Entity,
			EntityID:  e.EntityID,
			ActorID:   e.ActorUserID,
			CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}
