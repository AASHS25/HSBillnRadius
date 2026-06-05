package billingsvc_test

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/billingsvc"
)

const tenantID = int64(1)

// fakeRestorer implements Restorer, Isolator and Commissioner for billing tests.
type fakeRestorer struct {
	calls        []string
	isolateCalls []string
	commissions  []int64
}

func (f *fakeRestorer) Restore(_ context.Context, _ int64, username, planGroup string) error {
	f.calls = append(f.calls, username+"|"+planGroup)
	return nil
}

func (f *fakeRestorer) Isolate(_ context.Context, _ int64, username, isolirGroup string) error {
	f.isolateCalls = append(f.isolateCalls, username+"|"+isolirGroup)
	return nil
}

func (f *fakeRestorer) Record(_ context.Context, _, _, _, amountIDR int64) error {
	f.commissions = append(f.commissions, amountIDR)
	return nil
}

func setup(t *testing.T) (*billingsvc.Service, *memrepo.Store, *fakeRestorer, int64, plan.Plan) {
	t.Helper()
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()

	p, err := r.Plan.Create(ctx, plan.Plan{
		TenantID: tenantID, Name: "Home", ServiceType: plan.ServicePPPoE,
		PriceIDR: 300000, TaxBps: 0, ActiveDays: 30, BillingCycle: plan.CycleMonthly,
	})
	require.NoError(t, err)

	install := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	c, err := r.Customer.Create(ctx, customer.Customer{
		TenantID: tenantID, CustomerNo: "C1", Name: "Budi", Status: customer.StatusActive,
		PlanID: &p.ID, PppoeUsername: "budi", InstallDate: &install,
	})
	require.NoError(t, err)

	restorer := &fakeRestorer{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return billingsvc.New(r, store, restorer, log), store, restorer, c.ID, p
}

func june() time.Time { return time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC) }

func TestGenerateMonthly_FullAndIdempotent(t *testing.T) {
	svc, _, _, custID, _ := setup(t)

	inv, err := svc.GenerateMonthly(context.Background(), tenantID, custID, 1, june())
	require.NoError(t, err)
	assert.Equal(t, int64(300000), inv.TotalIDR)
	assert.Equal(t, billing.StatusUnpaid, inv.Status)
	assert.Equal(t, "INV-202606-"+strconv.FormatInt(custID, 10), inv.InvoiceNo)

	_, err = svc.GenerateMonthly(context.Background(), tenantID, custID, 1, june())
	assert.ErrorIs(t, err, billing.ErrInvoiceExists)
}

func TestPayInvoice_SettlesExtendsAndRestores(t *testing.T) {
	svc, store, restorer, custID, p := setup(t)
	ctx := context.Background()

	inv, err := svc.GenerateMonthly(ctx, tenantID, custID, 1, june())
	require.NoError(t, err)

	pay, err := svc.PayInvoice(ctx, tenantID, inv.ID, 7, billing.MethodCash)
	require.NoError(t, err)
	assert.Equal(t, billing.PaySettled, pay.Status)
	assert.Equal(t, int64(300000), pay.AmountIDR)

	// Invoice is now paid.
	got, err := svc.Get(ctx, tenantID, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, billing.StatusPaid, got.Status)
	assert.Len(t, got.Items, 1)

	// RADIUS restore was triggered into the plan group.
	assert.Equal(t, []string{"budi|" + p.GroupName()}, restorer.calls)

	// Service extended ~30 days into the future.
	c, err := store.Repositories().Customer.GetByID(ctx, tenantID, custID)
	require.NoError(t, err)
	require.NotNil(t, c.ActiveUntil)
	assert.True(t, c.ActiveUntil.After(time.Now().AddDate(0, 0, 25)))

	// Report reflects the income and zero outstanding.
	rep, err := svc.Report(ctx, tenantID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(300000), rep.Income)
	assert.Equal(t, int64(0), rep.Outstanding)
}

func TestPayInvoice_AlreadyPaid(t *testing.T) {
	svc, _, _, custID, _ := setup(t)
	ctx := context.Background()
	inv, err := svc.GenerateMonthly(ctx, tenantID, custID, 1, june())
	require.NoError(t, err)
	_, err = svc.PayInvoice(ctx, tenantID, inv.ID, 1, billing.MethodCash)
	require.NoError(t, err)

	_, err = svc.PayInvoice(ctx, tenantID, inv.ID, 1, billing.MethodCash)
	assert.ErrorIs(t, err, billing.ErrAlreadyPaid)
}

func TestPayInvoice_RecordsResellerCommission(t *testing.T) {
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()
	resellerID := int64(99)
	p, err := r.Plan.Create(ctx, plan.Plan{TenantID: tenantID, Name: "P", ServiceType: plan.ServicePPPoE, PriceIDR: 100000, ActiveDays: 30})
	require.NoError(t, err)
	c, err := r.Customer.Create(ctx, customer.Customer{
		TenantID: tenantID, CustomerNo: "C9", Name: "Ratna", Status: customer.StatusActive,
		PlanID: &p.ID, ResellerID: &resellerID,
	})
	require.NoError(t, err)

	fake := &fakeRestorer{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// 5% commission.
	svc := billingsvc.New(r, store, fake, log).WithCommission(500, fake)

	inv, err := svc.GenerateMonthly(ctx, tenantID, c.ID, 1, june())
	require.NoError(t, err)
	_, err = svc.PayInvoice(ctx, tenantID, inv.ID, 1, billing.MethodCash)
	require.NoError(t, err)

	// 5% of the invoice total is recorded as commission.
	require.Len(t, fake.commissions, 1)
	assert.Equal(t, inv.TotalIDR*500/10000, fake.commissions[0])
}

func TestRunIsolirScan_IsolatesExpired(t *testing.T) {
	svc, store, fake, custID, _ := setup(t)
	ctx := context.Background()

	// Expire the customer's service.
	past := time.Now().Add(-time.Hour)
	require.NoError(t, store.Repositories().Customer.SetActiveUntil(ctx, tenantID, custID, past, customer.StatusActive))

	n, err := svc.RunIsolirScan(ctx, fake, 100)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	c, err := store.Repositories().Customer.GetByID(ctx, tenantID, custID)
	require.NoError(t, err)
	assert.Equal(t, customer.StatusIsolated, c.Status)
	assert.Equal(t, []string{"budi|isolir"}, fake.isolateCalls)

	// A second scan finds nothing (already isolated).
	n, err = svc.RunIsolirScan(ctx, fake, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
