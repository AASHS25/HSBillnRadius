package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type ticketRepo struct{ q *sqlc.Queries }

func toDomainTicket(m sqlc.Ticket) ticket.Ticket {
	return ticket.Ticket{
		ID: m.ID, TenantID: m.TenantID, CustomerID: int8Ptr(m.CustomerID), Type: ticket.Type(m.Type),
		Subject: m.Subject, Description: m.Description, Status: ticket.Status(m.Status), Priority: m.Priority,
		AssignedUserID: int8Ptr(m.AssignedUserID), CreatedBy: int8Ptr(m.CreatedBy),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, ResolvedAt: tsPtr(m.ResolvedAt),
	}
}

func (r *ticketRepo) Create(ctx context.Context, t ticket.Ticket) (ticket.Ticket, error) {
	m, err := r.q.CreateTicket(ctx, sqlc.CreateTicketParams{
		TenantID: t.TenantID, CustomerID: pgInt8Ptr(t.CustomerID), Type: sqlc.TicketType(t.Type),
		Subject: t.Subject, Description: t.Description, Priority: t.Priority,
		AssignedUserID: pgInt8Ptr(t.AssignedUserID), CreatedBy: pgInt8Ptr(t.CreatedBy),
	})
	if err != nil {
		return ticket.Ticket{}, fmt.Errorf("create ticket: %w", err)
	}
	return toDomainTicket(m), nil
}

func (r *ticketRepo) Get(ctx context.Context, tenantID, id int64) (ticket.Ticket, error) {
	m, err := r.q.GetTicket(ctx, sqlc.GetTicketParams{ID: id, TenantID: tenantID})
	if err != nil {
		if isNotFound(err) {
			return ticket.Ticket{}, ticket.ErrNotFound
		}
		return ticket.Ticket{}, fmt.Errorf("get ticket: %w", err)
	}
	return toDomainTicket(m), nil
}

func (r *ticketRepo) List(ctx context.Context, tenantID int64, limit, offset int32) ([]ticket.Ticket, error) {
	rows, err := r.q.ListTickets(ctx, sqlc.ListTicketsParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	out := make([]ticket.Ticket, len(rows))
	for i, m := range rows {
		out[i] = toDomainTicket(m)
	}
	return out, nil
}

func (r *ticketRepo) UpdateStatus(ctx context.Context, tenantID, id int64, status ticket.Status) error {
	if err := r.q.UpdateTicketStatus(ctx, sqlc.UpdateTicketStatusParams{ID: id, TenantID: tenantID, Status: sqlc.TicketStatus(status)}); err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}
	return nil
}

func (r *ticketRepo) AddEvent(ctx context.Context, e ticket.Event) (ticket.Event, error) {
	m, err := r.q.AddTicketEvent(ctx, sqlc.AddTicketEventParams{
		TicketID: e.TicketID, UserID: pgInt8Ptr(e.UserID), Note: e.Note, StatusFrom: e.StatusFrom, StatusTo: e.StatusTo,
	})
	if err != nil {
		return ticket.Event{}, fmt.Errorf("add ticket event: %w", err)
	}
	return ticket.Event{
		ID: m.ID, TicketID: m.TicketID, UserID: int8Ptr(m.UserID), Note: m.Note,
		StatusFrom: m.StatusFrom, StatusTo: m.StatusTo, CreatedAt: m.CreatedAt,
	}, nil
}

func (r *ticketRepo) ListEvents(ctx context.Context, ticketID int64) ([]ticket.Event, error) {
	rows, err := r.q.ListTicketEvents(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list ticket events: %w", err)
	}
	out := make([]ticket.Event, len(rows))
	for i, m := range rows {
		out[i] = ticket.Event{
			ID: m.ID, TicketID: m.TicketID, UserID: int8Ptr(m.UserID), Note: m.Note,
			StatusFrom: m.StatusFrom, StatusTo: m.StatusTo, CreatedAt: m.CreatedAt,
		}
	}
	return out, nil
}
