package customersvc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/customersvc"
)

const tenantID = int64(1)

func newService() (*customersvc.Service, *memrepo.Store) {
	store := memrepo.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return customersvc.New(store.Repositories(), store, log), store
}

// seedPlan inserts a plan directly via the repo and returns it.
func seedPlan(t *testing.T, store *memrepo.Store, p plan.Plan) plan.Plan {
	t.Helper()
	p.TenantID = tenantID
	created, err := store.Repositories().Plan.Create(context.Background(), p)
	require.NoError(t, err)
	return created
}

func TestCreate_ProvisionsRadiusForPPPoE(t *testing.T) {
	svc, store := newService()
	p := seedPlan(t, store, plan.Plan{Name: "P10", ServiceType: plan.ServicePPPoE})

	c, err := svc.Create(context.Background(), tenantID, 7, customersvc.Input{
		CustomerNo:    "C-001",
		Name:          "Budi",
		Status:        customer.StatusActive,
		PlanID:        &p.ID,
		PppoeUsername: "budi",
		PppoePassword: "secret1",
	})
	require.NoError(t, err)
	assert.Positive(t, c.ID)

	pw, ok := store.RadCheckValue(tenantID, "budi", "Cleartext-Password")
	require.True(t, ok)
	assert.Equal(t, "secret1", pw)

	group, ok := store.UserGroupName(tenantID, "budi")
	require.True(t, ok)
	assert.Equal(t, p.GroupName(), group)
}

func TestCreate_NoCredentials_NoRadius(t *testing.T) {
	svc, store := newService()
	_, err := svc.Create(context.Background(), tenantID, 0, customersvc.Input{
		CustomerNo: "C-002", Name: "NoNet", Status: customer.StatusNew,
	})
	require.NoError(t, err)
	_, ok := store.UserGroupName(tenantID, "")
	assert.False(t, ok)
}

func TestCreate_DuplicateCustomerNo(t *testing.T) {
	svc, _ := newService()
	in := customersvc.Input{CustomerNo: "DUP", Name: "A", Status: customer.StatusNew}
	_, err := svc.Create(context.Background(), tenantID, 0, in)
	require.NoError(t, err)
	in.Name = "B"
	_, err = svc.Create(context.Background(), tenantID, 0, in)
	assert.ErrorIs(t, err, customer.ErrNoTaken)
}

func TestUpdate_UsernameChange_ClearsOld(t *testing.T) {
	svc, store := newService()
	p := seedPlan(t, store, plan.Plan{Name: "P", ServiceType: plan.ServicePPPoE})

	c, err := svc.Create(context.Background(), tenantID, 0, customersvc.Input{
		CustomerNo: "C-003", Name: "Citra", Status: customer.StatusActive,
		PlanID: &p.ID, PppoeUsername: "citra", PppoePassword: "p1",
	})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), tenantID, 0, c.ID, customersvc.Input{
		CustomerNo: "C-003", Name: "Citra", Status: customer.StatusActive,
		PlanID: &p.ID, PppoeUsername: "citra2", PppoePassword: "p1",
	})
	require.NoError(t, err)

	_, oldExists := store.UserGroupName(tenantID, "citra")
	assert.False(t, oldExists, "old username radius rows should be cleared")
	group, ok := store.UserGroupName(tenantID, "citra2")
	require.True(t, ok)
	assert.Equal(t, p.GroupName(), group)
}

func TestCreate_IsolatedUsesIsolirGroup(t *testing.T) {
	svc, store := newService()
	isolirID := int64(999)
	p := seedPlan(t, store, plan.Plan{Name: "P", ServiceType: plan.ServicePPPoE, IsolirPlanID: &isolirID})

	_, err := svc.Create(context.Background(), tenantID, 0, customersvc.Input{
		CustomerNo: "C-004", Name: "Dewi", Status: customer.StatusIsolated,
		PlanID: &p.ID, PppoeUsername: "dewi", PppoePassword: "p1",
	})
	require.NoError(t, err)

	group, ok := store.UserGroupName(tenantID, "dewi")
	require.True(t, ok)
	assert.Equal(t, "plan_999", group, "isolated customer should land in the isolir group")
}

func TestDelete_ClearsRadius(t *testing.T) {
	svc, store := newService()
	p := seedPlan(t, store, plan.Plan{Name: "P", ServiceType: plan.ServicePPPoE})
	c, err := svc.Create(context.Background(), tenantID, 0, customersvc.Input{
		CustomerNo: "C-005", Name: "Eka", Status: customer.StatusActive,
		PlanID: &p.ID, PppoeUsername: "eka", PppoePassword: "p1",
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), tenantID, 0, c.ID))
	_, ok := store.UserGroupName(tenantID, "eka")
	assert.False(t, ok)
}
