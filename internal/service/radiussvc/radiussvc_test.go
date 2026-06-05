package radiussvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainradius "github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
)

func newService() (*radiussvc.Service, *memrepo.Store) {
	store := memrepo.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := radiussvc.New(store.Repositories(), cache.Noop{}, time.Minute, time.Minute, log)
	return svc, store
}

// provision seeds a user with a password, group and group reply.
func provision(t *testing.T, store *memrepo.Store) {
	t.Helper()
	r := store.Repositories()
	ctx := context.Background()
	require.NoError(t, r.RadiusMap.SetUserPassword(ctx, 1, "alice", "wonderland"))
	require.NoError(t, r.RadiusMap.SetUserGroup(ctx, 1, "alice", "plan_1", 1))
	require.NoError(t, r.RadiusMap.SetGroupReply(ctx, 1, "plan_1", []domainradius.Attr{
		{Attribute: domainradius.AttrMikrotikRateLimit, Op: ":=", Value: "10M/2M"},
		{Attribute: domainradius.AttrFramedPool, Op: ":=", Value: "pool-a"},
	}))
}

func TestResolveNAS(t *testing.T) {
	svc, store := newService()
	store.SeedNas(domainradius.Nas{TenantID: 1, Name: "10.0.0.1", Secret: "s3cret"})

	nas, err := svc.ResolveNAS(context.Background(), "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), nas.TenantID)
	assert.Equal(t, "s3cret", nas.Secret)

	_, err = svc.ResolveNAS(context.Background(), "10.0.0.99")
	assert.True(t, radiussvc.IsNasUnknown(err))
}

func TestLookup_FoundWithReply(t *testing.T) {
	svc, store := newService()
	provision(t, store)

	info, err := svc.Lookup(context.Background(), 1, "alice")
	require.NoError(t, err)
	assert.True(t, info.Found)
	assert.Equal(t, "wonderland", info.ClearPassword)

	var rate string
	for _, a := range info.Reply {
		if a.Attribute == domainradius.AttrMikrotikRateLimit {
			rate = a.Value
		}
	}
	assert.Equal(t, "10M/2M", rate)
}

func TestLookup_NotFound(t *testing.T) {
	svc, _ := newService()
	info, err := svc.Lookup(context.Background(), 1, "ghost")
	require.NoError(t, err)
	assert.False(t, info.Found)
}
