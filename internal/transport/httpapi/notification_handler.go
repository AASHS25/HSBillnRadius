package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/notification"
)

type gatewayRequest struct {
	Provider   string            `json:"provider" validate:"required,oneof=fonnte wablas starsender onesender unofficial"`
	Config     map[string]string `json:"config" validate:"required"`
	IsActive   bool              `json:"is_active"`
	DailyLimit int               `json:"daily_limit" validate:"gte=0"`
}

type templateRequest struct {
	Key     string `json:"key" validate:"required,max=64"`
	Channel string `json:"channel" validate:"omitempty,oneof=wa email"`
	Body    string `json:"body" validate:"required,max=4000"`
	Active  bool   `json:"is_active"`
}

type sendNotificationRequest struct {
	To          string            `json:"to" validate:"required"`
	TemplateKey string            `json:"template_key" validate:"required"`
	Vars        map[string]string `json:"vars"`
	DedupKey    string            `json:"dedup_key"`
}

func (a *API) handleConfigureGateway(w http.ResponseWriter, r *http.Request) {
	var req gatewayRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	g, err := a.notify.ConfigureGateway(r.Context(), notification.Gateway{
		TenantID:   TenantID(r.Context()),
		Provider:   notification.Provider(req.Provider),
		Config:     req.Config,
		IsActive:   req.IsActive,
		DailyLimit: req.DailyLimit,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": g.ID, "provider": string(g.Provider)})
}

func (a *API) handleUpsertTemplate(w http.ResponseWriter, r *http.Request) {
	var req templateRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	channel := notification.Channel(req.Channel)
	if channel == "" {
		channel = notification.ChannelWA
	}
	t, err := a.notify.UpsertTemplate(r.Context(), notification.Template{
		TenantID: TenantID(r.Context()),
		Key:      req.Key,
		Channel:  channel,
		Body:     req.Body,
		IsActive: req.Active,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": t.ID, "key": t.Key})
}

func (a *API) handleSendNotification(w http.ResponseWriter, r *http.Request) {
	var req sendNotificationRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	if err := a.notify.Enqueue(r.Context(), notification.Job{
		TenantID:    TenantID(r.Context()),
		Channel:     notification.ChannelWA,
		To:          req.To,
		TemplateKey: req.TemplateKey,
		Vars:        req.Vars,
		DedupKey:    req.DedupKey,
	}); err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}
