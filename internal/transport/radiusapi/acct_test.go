package radiusapi_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
	"github.com/aashs25/hsbillnradius/internal/transport/radiusapi"
)

func acctRequest(t *testing.T, status rfc2866.AcctStatusType, user, sessionID string) []byte {
	t.Helper()
	p := radius.New(radius.CodeAccountingRequest, []byte(secret))
	require.NoError(t, rfc2866.AcctStatusType_Set(p, status))
	require.NoError(t, rfc2865.UserName_SetString(p, user))
	require.NoError(t, rfc2866.AcctSessionID_SetString(p, sessionID))
	raw, err := p.Encode()
	require.NoError(t, err)
	return raw
}

func TestAcctHandle_StartThenStop(t *testing.T) {
	store := memrepo.New()
	store.SeedNas(domainradius.Nas{TenantID: 1, Name: nasIP, Secret: secret})
	repos := store.Repositories()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := radiussvc.New(repos, cache.Noop{}, 0, 0, log)
	writer := radiussvc.NewAcctWriter(repos, 16, log)
	h := radiusapi.NewAcctHandler(svc, writer, log)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- writer.Run(ctx) }()

	// Start
	reply, err := h.Handle(context.Background(), remote(nasIP), acctRequest(t, rfc2866.AcctStatusType_Value_Start, "bob", "sess-1"))
	require.NoError(t, err)
	assert.Equal(t, radius.CodeAccountingResponse, parseReply(t, reply).Code)

	// Stop
	_, err = h.Handle(context.Background(), remote(nasIP), acctRequest(t, rfc2866.AcctStatusType_Value_Stop, "bob", "sess-1"))
	require.NoError(t, err)

	cancel()
	require.NoError(t, <-done)

	active, err := repos.Accounting.ActiveSessions(context.Background(), 1, "bob")
	require.NoError(t, err)
	assert.Empty(t, active, "session closed after Stop accounting")
}

func TestAcctHandle_UnknownNAS_Drop(t *testing.T) {
	store := memrepo.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := radiussvc.New(store.Repositories(), cache.Noop{}, 0, 0, log)
	writer := radiussvc.NewAcctWriter(store.Repositories(), 4, log)
	h := radiusapi.NewAcctHandler(svc, writer, log)

	reply, err := h.Handle(context.Background(), remote("203.0.113.5"), acctRequest(t, rfc2866.AcctStatusType_Value_Start, "x", "s"))
	require.NoError(t, err)
	assert.Nil(t, reply)
}
