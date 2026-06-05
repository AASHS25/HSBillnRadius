package plansvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/plansvc"
)

func newService() (*plansvc.Service, *memrepo.Store) {
	store := memrepo.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return plansvc.New(store.Repositories(), store, log), store
}

func basicInput() plansvc.Input {
	return plansvc.Input{
		Name:         "Home 10M",
		ServiceType:  plan.ServicePPPoE,
		PriceIDR:     100000,
		TaxBps:       1100, // 11%
		BillingCycle: plan.CycleMonthly,
		ActiveDays:   30,
		IsUnlimited:  true,
		PoolName:     "pool-home",
		Bandwidth:    plansvc.BandwidthInput{RateLimitRx: "10M", RateLimitTx: "2M"},
	}
}

func TestCreate_SyncsRadiusGroupAndTax(t *testing.T) {
	svc, store := newService()

	res, err := svc.Create(context.Background(), 1, 5, basicInput())
	require.NoError(t, err)
	assert.Positive(t, res.Plan.ID)
	assert.Equal(t, "plan_1", res.Plan.GroupName())
	assert.Equal(t, int64(11000), res.Plan.TaxIDR()) // 11% of 100000
	assert.Equal(t, int64(111000), res.Plan.TotalIDR())

	rate, ok := store.GroupReplyValue(1, "plan_1", "Mikrotik-Rate-Limit")
	require.True(t, ok)
	assert.Equal(t, "10M/2M", rate)
	pool, ok := store.GroupReplyValue(1, "plan_1", "Framed-Pool")
	require.True(t, ok)
	assert.Equal(t, "pool-home", pool)
}

func TestCreate_InvalidServiceType(t *testing.T) {
	svc, _ := newService()
	in := basicInput()
	in.ServiceType = "satellite"
	_, err := svc.Create(context.Background(), 1, 0, in)
	assert.ErrorIs(t, err, plan.ErrInvalidPlan)
}

func TestCreate_DuplicateName(t *testing.T) {
	svc, _ := newService()
	_, err := svc.Create(context.Background(), 1, 0, basicInput())
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), 1, 0, basicInput())
	assert.ErrorIs(t, err, plan.ErrNameTaken)
}

func TestUpdate_RewritesRadiusGroup(t *testing.T) {
	svc, store := newService()
	res, err := svc.Create(context.Background(), 1, 0, basicInput())
	require.NoError(t, err)

	in := basicInput()
	in.Bandwidth = plansvc.BandwidthInput{RateLimitRx: "20M", RateLimitTx: "5M"}
	_, err = svc.Update(context.Background(), 1, 0, res.Plan.ID, in)
	require.NoError(t, err)

	rate, ok := store.GroupReplyValue(1, "plan_1", "Mikrotik-Rate-Limit")
	require.True(t, ok)
	assert.Equal(t, "20M/5M", rate)
}

func TestDelete_ClearsGroupReply(t *testing.T) {
	svc, store := newService()
	res, err := svc.Create(context.Background(), 1, 0, basicInput())
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), 1, 0, res.Plan.ID))

	_, ok := store.GroupReplyValue(1, "plan_1", "Mikrotik-Rate-Limit")
	assert.False(t, ok, "group reply should be cleared on delete")

	_, err = svc.Get(context.Background(), 1, res.Plan.ID)
	assert.ErrorIs(t, err, plan.ErrNotFound)
}
