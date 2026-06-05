package resellersvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/iam"
	"github.com/aashs25/hsbillnradius/internal/domain/reseller"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/resellersvc"
)

func newSvc(t *testing.T) (*resellersvc.Service, *memrepo.Store, int64) {
	t.Helper()
	store := memrepo.New()
	u, err := store.Repositories().User.Create(context.Background(), iam.User{TenantID: 1, Name: "Reseller", Email: "r@x.test"})
	require.NoError(t, err)
	return resellersvc.New(store.Repositories(), store, slog.New(slog.NewTextHandler(io.Discard, nil))), store, u.ID
}

func TestTopup_AccumulatesBalance(t *testing.T) {
	svc, store, uid := newSvc(t)
	ctx := context.Background()

	dep, bal, err := svc.Topup(ctx, 1, uid, 100000, "transfer")
	require.NoError(t, err)
	assert.EqualValues(t, 100000, dep.AmountIDR)
	assert.EqualValues(t, 100000, bal)

	_, bal2, err := svc.Topup(ctx, 1, uid, 50000, "cash")
	require.NoError(t, err)
	assert.EqualValues(t, 150000, bal2)

	deposits, err := svc.ListDeposits(ctx, 1, uid, 50, 0)
	require.NoError(t, err)
	assert.Len(t, deposits, 2)

	u, _ := store.Repositories().User.GetByID(ctx, uid)
	assert.EqualValues(t, 150000, u.BalanceIDR)
}

func TestTopup_InvalidAmount(t *testing.T) {
	svc, _, uid := newSvc(t)
	_, _, err := svc.Topup(context.Background(), 1, uid, 0, "cash")
	assert.ErrorIs(t, err, reseller.ErrInvalidAmount)
}

func TestRecord_CreatesCommissionAndCredits(t *testing.T) {
	svc, store, uid := newSvc(t)
	ctx := context.Background()

	require.NoError(t, svc.Record(ctx, 1, uid, 42, 5000))

	commissions, err := svc.ListCommissions(ctx, 1, uid, 50, 0)
	require.NoError(t, err)
	require.Len(t, commissions, 1)
	assert.EqualValues(t, 5000, commissions[0].AmountIDR)

	u, _ := store.Repositories().User.GetByID(ctx, uid)
	assert.EqualValues(t, 5000, u.BalanceIDR)
}
