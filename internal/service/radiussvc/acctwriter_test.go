package radiussvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
)

func TestAcctWriter_AppliesStartThenStop(t *testing.T) {
	store := memrepo.New()
	repos := store.Repositories()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := radiussvc.NewAcctWriter(repos, 16, log)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	uid := domainradius.ComputeUniqueID("s1", "10.0.0.1", "bob")
	require.True(t, w.Submit(domainradius.AcctEvent{
		TenantID: 1, Status: domainradius.AcctStart, SessionID: "s1", UniqueID: uid,
		Username: "bob", NASIP: "10.0.0.1",
	}))
	require.True(t, w.Submit(domainradius.AcctEvent{
		TenantID: 1, Status: domainradius.AcctStop, UniqueID: uid,
		SessionTime: 120, InputOctets: 1000, OutputOctets: 2000, TerminateCause: "1",
	}))

	cancel() // triggers drain of any buffered events
	require.NoError(t, <-done)

	// After Start+Stop the session is no longer active.
	active, err := repos.Accounting.ActiveSessions(context.Background(), 1, "bob")
	require.NoError(t, err)
	assert.Empty(t, active, "session should be closed after Stop")
}

func TestAcctWriter_SubmitDropsWhenFull(t *testing.T) {
	store := memrepo.New()
	w := radiussvc.NewAcctWriter(store.Repositories(), 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// No consumer running: the single slot fills, then submits are dropped.
	assert.True(t, w.Submit(domainradius.AcctEvent{Status: domainradius.AcctStart}))
	assert.False(t, w.Submit(domainradius.AcctEvent{Status: domainradius.AcctStart}))
}
