package ticketsvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/ticket"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/ticketsvc"
)

func newSvc() *ticketsvc.Service {
	store := memrepo.New()
	return ticketsvc.New(store.Repositories(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateAndTransition(t *testing.T) {
	svc := newSvc()
	ctx := context.Background()

	tk, err := svc.Create(ctx, 1, 9, ticketsvc.CreateInput{Type: ticket.TypeTrouble, Subject: "No internet"})
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusOpen, tk.Status)

	tk, err = svc.Transition(ctx, 1, 9, tk.ID, ticket.StatusInProgress, "technician assigned")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusInProgress, tk.Status)

	tk, err = svc.Transition(ctx, 1, 9, tk.ID, ticket.StatusResolved, "fixed")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusResolved, tk.Status)

	// Timeline records open + 2 transitions.
	_, events, err := svc.Get(ctx, 1, tk.ID)
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestTransition_Invalid(t *testing.T) {
	svc := newSvc()
	ctx := context.Background()
	tk, err := svc.Create(ctx, 1, 0, ticketsvc.CreateInput{Type: ticket.TypeInstall, Subject: "New install"})
	require.NoError(t, err)

	_, err = svc.Transition(ctx, 1, 0, tk.ID, ticket.StatusClosed, "")
	require.NoError(t, err)
	// closed is terminal.
	_, err = svc.Transition(ctx, 1, 0, tk.ID, ticket.StatusOpen, "")
	assert.ErrorIs(t, err, ticket.ErrInvalidStatus)
}

func TestCreate_Validation(t *testing.T) {
	svc := newSvc()
	_, err := svc.Create(context.Background(), 1, 0, ticketsvc.CreateInput{Type: ticket.TypeTrouble, Subject: ""})
	assert.ErrorIs(t, err, ticket.ErrInvalidTicket)
}
