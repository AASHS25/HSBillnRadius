package coasvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/coasvc"
)

type fakeDialer struct{ calls []string }

func (f *fakeDialer) Disconnect(_ context.Context, nas domainradius.Nas, username, sessionID string) error {
	f.calls = append(f.calls, nas.Name+"|"+username+"|"+sessionID)
	return nil
}

func setup(t *testing.T) (*coasvc.Service, *memrepo.Store, *fakeDialer) {
	t.Helper()
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()

	store.SeedNas(domainradius.Nas{TenantID: 1, Name: "10.0.0.1", Secret: "s"})
	require.NoError(t, r.RadiusMap.SetUserGroup(ctx, 1, "alice", "plan_1", 1))
	require.NoError(t, r.Accounting.Start(ctx, domainradius.AcctEvent{
		TenantID: 1, Status: domainradius.AcctStart, SessionID: "sess1",
		UniqueID: domainradius.ComputeUniqueID("sess1", "10.0.0.1", "alice"),
		Username: "alice", NASIP: "10.0.0.1",
	}))

	dialer := &fakeDialer{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return coasvc.New(r, dialer, cache.Noop{}, log), store, dialer
}

func TestIsolate_SwitchesGroupAndDisconnects(t *testing.T) {
	svc, store, dialer := setup(t)

	require.NoError(t, svc.Isolate(context.Background(), 1, "alice", "plan_isolir"))

	group, ok := store.UserGroupName(1, "alice")
	require.True(t, ok)
	assert.Equal(t, "plan_isolir", group)
	assert.Equal(t, []string{"10.0.0.1|alice|sess1"}, dialer.calls, "active session should be kicked")
}

func TestRestore_SwitchesBackAndDisconnects(t *testing.T) {
	svc, store, dialer := setup(t)

	require.NoError(t, svc.Restore(context.Background(), 1, "alice", "plan_1"))

	group, _ := store.UserGroupName(1, "alice")
	assert.Equal(t, "plan_1", group)
	assert.Len(t, dialer.calls, 1)
}
