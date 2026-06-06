package httpapi

import "net/http"

type nasRequest struct {
	Nasname   string `json:"nasname" validate:"required,max=128"`
	Shortname string `json:"shortname" validate:"max=64"`
	Secret    string `json:"secret" validate:"required,min=4,max=128"`
}

func (a *API) handleCreateNas(w http.ResponseWriter, r *http.Request) {
	var req nasRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	nas, err := a.nas.Create(r.Context(), TenantID(r.Context()), req.Nasname, req.Shortname, req.Secret)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": nas.ID, "nasname": nas.Name, "shortname": nas.Shortname})
}

func (a *API) handleListNas(w http.ResponseWriter, r *http.Request) {
	devices, err := a.nas.List(r.Context(), TenantID(r.Context()))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	// The shared secret is never returned to clients.
	items := make([]map[string]any, len(devices))
	for i, n := range devices {
		items[i] = map[string]any{"id": n.ID, "nasname": n.Name, "shortname": n.Shortname}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
