package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aashs25/hsbillnradius/internal/ports/acs"
)

type setWiFiRequest struct {
	SSID     string `json:"ssid" validate:"required,max=64"`
	Password string `json:"password" validate:"required,min=8,max=64"`
}

type registerDeviceRequest struct {
	DeviceID   string `json:"device_id" validate:"required"`
	CustomerID *int64 `json:"customer_id"`
}

func storedDeviceDTO(d acs.StoredDevice) map[string]any {
	return map[string]any{
		"device_id": d.DeviceID, "serial_number": d.SerialNumber, "manufacturer": d.Manufacturer,
		"model": d.Model, "wan_ip": d.WANIP, "ssid": d.SSID, "customer_id": d.CustomerID,
	}
}

func (a *API) handleListACSDevices(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	devices, err := a.acs.ListDevices(r.Context(), TenantID(r.Context()), limit, offset)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	items := make([]map[string]any, len(devices))
	for i, d := range devices {
		items[i] = storedDeviceDTO(d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *API) handleRegisterACSDevice(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	dev, err := a.acs.GetDevice(r.Context(), TenantID(r.Context()), req.DeviceID, req.CustomerID)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id": dev.ID, "serial_number": dev.SerialNumber, "model": dev.Model,
		"wan_ip": dev.WANIP, "ssid": dev.SSID, "last_inform": dev.LastInform,
	})
}

func (a *API) handleSetACSWiFi(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	var req setWiFiRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.acs.SetWiFi(r.Context(), deviceID, req.SSID, req.Password); err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "task_queued"})
}

func (a *API) handleRebootACSDevice(w http.ResponseWriter, r *http.Request) {
	if err := a.acs.Reboot(r.Context(), chi.URLParam(r, "deviceID")); err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "task_queued"})
}
