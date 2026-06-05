// Package ticketsvc implements support-ticket use-cases with a guarded status
// machine and a timeline of events.
package ticketsvc

import (
	"context"
	"log/slog"

	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Service provides ticket operations.
type Service struct {
	repos repo.Repositories
	log   *slog.Logger
}

// New builds a ticket Service.
func New(repos repo.Repositories, log *slog.Logger) *Service {
	return &Service{repos: repos, log: log}
}

// CreateInput is a new-ticket request.
type CreateInput struct {
	CustomerID     *int64
	Type           ticket.Type
	Subject        string
	Description    string
	Priority       int32
	AssignedUserID *int64
}

// Create opens a new ticket and records the opening event.
func (s *Service) Create(ctx context.Context, tenantID, actorID int64, in CreateInput) (ticket.Ticket, error) {
	t := ticket.Ticket{
		TenantID: tenantID, CustomerID: in.CustomerID, Type: in.Type, Subject: in.Subject,
		Description: in.Description, Priority: in.Priority, AssignedUserID: in.AssignedUserID,
		CreatedBy: actorPtr(actorID), Status: ticket.StatusOpen,
	}
	if t.Priority == 0 {
		t.Priority = 3
	}
	if err := t.Validate(); err != nil {
		return ticket.Ticket{}, err
	}
	created, err := s.repos.Ticket.Create(ctx, t)
	if err != nil {
		return ticket.Ticket{}, err
	}
	if _, err := s.repos.Ticket.AddEvent(ctx, ticket.Event{
		TicketID: created.ID, UserID: actorPtr(actorID), Note: "ticket opened", StatusTo: string(ticket.StatusOpen),
	}); err != nil {
		s.log.WarnContext(ctx, "add ticket open event failed", slog.Any("error", err))
	}
	return created, nil
}

// Get returns a ticket with its event timeline.
func (s *Service) Get(ctx context.Context, tenantID, id int64) (ticket.Ticket, []ticket.Event, error) {
	t, err := s.repos.Ticket.Get(ctx, tenantID, id)
	if err != nil {
		return ticket.Ticket{}, nil, err
	}
	events, err := s.repos.Ticket.ListEvents(ctx, id)
	if err != nil {
		return ticket.Ticket{}, nil, err
	}
	return t, events, nil
}

// List returns a page of tickets.
func (s *Service) List(ctx context.Context, tenantID int64, limit, offset int32) ([]ticket.Ticket, error) {
	return s.repos.Ticket.List(ctx, tenantID, limit, offset)
}

// Transition moves a ticket to a new status (guarded) and records the event.
func (s *Service) Transition(ctx context.Context, tenantID, actorID, id int64, to ticket.Status, note string) (ticket.Ticket, error) {
	t, err := s.repos.Ticket.Get(ctx, tenantID, id)
	if err != nil {
		return ticket.Ticket{}, err
	}
	if t.Status == to {
		return t, nil
	}
	if !ticket.CanTransition(t.Status, to) {
		return ticket.Ticket{}, ticket.ErrInvalidStatus
	}
	if err := s.repos.Ticket.UpdateStatus(ctx, tenantID, id, to); err != nil {
		return ticket.Ticket{}, err
	}
	if _, err := s.repos.Ticket.AddEvent(ctx, ticket.Event{
		TicketID: id, UserID: actorPtr(actorID), Note: note,
		StatusFrom: string(t.Status), StatusTo: string(to),
	}); err != nil {
		s.log.WarnContext(ctx, "add ticket transition event failed", slog.Any("error", err))
	}
	t.Status = to
	return t, nil
}

func actorPtr(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}
