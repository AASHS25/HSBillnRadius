package httpapi

import (
	"net/http"

	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/service/ticketsvc"
)

type createTicketRequest struct {
	CustomerID  *int64 `json:"customer_id"`
	Type        string `json:"type" validate:"required,oneof=trouble install other"`
	Subject     string `json:"subject" validate:"required,max=200"`
	Description string `json:"description" validate:"max=4000"`
	Priority    int32  `json:"priority" validate:"gte=0,lte=5"`
	AssignedTo  *int64 `json:"assigned_user_id"`
}

type transitionRequest struct {
	Status string `json:"status" validate:"required,oneof=open in_progress resolved closed"`
	Note   string `json:"note" validate:"max=2000"`
}

func ticketDTO(t ticket.Ticket) map[string]any {
	return map[string]any{
		"id": t.ID, "type": string(t.Type), "subject": t.Subject, "description": t.Description,
		"status": string(t.Status), "priority": t.Priority, "customer_id": t.CustomerID,
		"assigned_user_id": t.AssignedUserID, "created_at": t.CreatedAt,
	}
}

func (a *API) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	var req createTicketRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	t, err := a.tickets.Create(r.Context(), TenantID(r.Context()), UserID(r.Context()), ticketsvc.CreateInput{
		CustomerID: req.CustomerID, Type: ticket.Type(req.Type), Subject: req.Subject,
		Description: req.Description, Priority: req.Priority, AssignedUserID: req.AssignedTo,
	})
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticketDTO(t))
}

func (a *API) handleListTickets(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	tickets, err := a.tickets.List(r.Context(), TenantID(r.Context()), limit, offset)
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

func (a *API) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	t, events, err := a.tickets.Get(r.Context(), TenantID(r.Context()), id)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	dto := ticketDTO(t)
	evs := make([]map[string]any, len(events))
	for i, e := range events {
		evs[i] = map[string]any{"note": e.Note, "status_from": e.StatusFrom, "status_to": e.StatusTo, "created_at": e.CreatedAt}
	}
	dto["events"] = evs
	writeJSON(w, http.StatusOK, dto)
}

func (a *API) handleTransitionTicket(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	var req transitionRequest
	if err := decodeJSON(a.valid, w, r, &req); err != nil {
		writeError(w, a.log, err)
		return
	}
	t, err := a.tickets.Transition(r.Context(), TenantID(r.Context()), UserID(r.Context()), id, ticket.Status(req.Status), req.Note)
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	writeJSON(w, http.StatusOK, ticketDTO(t))
}

// handleMapsGeoJSON returns customers with coordinates as a GeoJSON
// FeatureCollection for the map view.
func (a *API) handleMapsGeoJSON(w http.ResponseWriter, r *http.Request) {
	locs, err := a.customers.Locations(r.Context(), TenantID(r.Context()))
	if err != nil {
		writeError(w, a.log, err)
		return
	}
	features := make([]map[string]any, len(locs))
	for i, l := range locs {
		features[i] = map[string]any{
			"type":     "Feature",
			"geometry": map[string]any{"type": "Point", "coordinates": []float64{l.Lng, l.Lat}},
			"properties": map[string]any{
				"id": l.ID, "name": l.Name, "customer_no": l.CustomerNo, "status": string(l.Status),
			},
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"type": "FeatureCollection", "features": features})
}
