// Package ticket holds the support-ticket entity and its status machine.
package ticket

import (
	"errors"
	"strings"
	"time"
)

// Type categorizes a ticket.
type Type string

const (
	TypeTrouble Type = "trouble"
	TypeInstall Type = "install"
	TypeOther   Type = "other"
)

// Valid reports whether t is a known type.
func (t Type) Valid() bool {
	switch t {
	case TypeTrouble, TypeInstall, TypeOther:
		return true
	default:
		return false
	}
}

// Status is a ticket lifecycle state.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("ticket not found")
	ErrInvalidTicket = errors.New("invalid ticket")
	ErrInvalidStatus = errors.New("invalid ticket status transition")
)

// Ticket is a support/trouble/install request.
type Ticket struct {
	ID             int64
	TenantID       int64
	CustomerID     *int64
	Type           Type
	Subject        string
	Description    string
	Status         Status
	Priority       int32
	AssignedUserID *int64
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ResolvedAt     *time.Time
}

// Event is an audit entry on a ticket's timeline.
type Event struct {
	ID         int64
	TicketID   int64
	UserID     *int64
	Note       string
	StatusFrom string
	StatusTo   string
	CreatedAt  time.Time
}

// transitions defines allowed status moves.
var transitions = map[Status]map[Status]bool{
	StatusOpen:       {StatusInProgress: true, StatusResolved: true, StatusClosed: true},
	StatusInProgress: {StatusResolved: true, StatusClosed: true, StatusOpen: true},
	StatusResolved:   {StatusClosed: true, StatusInProgress: true},
	StatusClosed:     {},
}

// CanTransition reports whether moving from->to is allowed.
func CanTransition(from, to Status) bool { return transitions[from][to] }

// Validate enforces ticket invariants on creation.
func (t Ticket) Validate() error {
	if strings.TrimSpace(t.Subject) == "" {
		return errors.Join(ErrInvalidTicket, errors.New("subject is required"))
	}
	if !t.Type.Valid() {
		return errors.Join(ErrInvalidTicket, errors.New("unknown type"))
	}
	return nil
}
