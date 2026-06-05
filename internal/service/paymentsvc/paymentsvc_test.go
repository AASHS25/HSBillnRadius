package paymentsvc_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/ports/payment"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/paymentsvc"
)

const (
	tenantID  = int64(1)
	serverKey = "SERVER-KEY"
	orderID   = "t1-inv1-123"
)

type fakeRestorer struct{ calls []string }

func (f *fakeRestorer) Restore(_ context.Context, _ int64, username, planGroup string) error {
	f.calls = append(f.calls, username+"|"+planGroup)
	return nil
}

type fakeNotifier struct{ jobs []notification.Job }

func (f *fakeNotifier) Enqueue(_ context.Context, job notification.Job) error {
	f.jobs = append(f.jobs, job)
	return nil
}

func midtransBody(orderID, statusCode, grossAmount, txStatus, key string) []byte {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + key))
	sig := hex.EncodeToString(sum[:])
	return []byte(fmt.Sprintf(
		`{"order_id":%q,"status_code":%q,"gross_amount":%q,"transaction_status":%q,"fraud_status":"accept","signature_key":%q}`,
		orderID, statusCode, grossAmount, txStatus, sig))
}

func setup(t *testing.T) (*paymentsvc.Service, *memrepo.Store, *fakeRestorer, *fakeNotifier) {
	t.Helper()
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()

	p, err := r.Plan.Create(ctx, plan.Plan{TenantID: tenantID, Name: "P", ServiceType: plan.ServicePPPoE, PriceIDR: 166500, ActiveDays: 30})
	require.NoError(t, err)
	c, err := r.Customer.Create(ctx, customer.Customer{
		TenantID: tenantID, CustomerNo: "C1", Name: "Budi", Status: customer.StatusIsolated,
		PlanID: &p.ID, PppoeUsername: "budi", PhoneWA: "628",
	})
	require.NoError(t, err)
	now := time.Now()
	inv, err := r.Billing.CreateInvoice(ctx, billing.Invoice{
		TenantID: tenantID, InvoiceNo: "INV1", CustomerID: c.ID, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0),
		TotalIDR: 166500, DueDate: now.AddDate(0, 0, 7), Status: billing.StatusUnpaid, Type: billing.TypePostpaid,
	})
	require.NoError(t, err)
	_, err = r.Billing.UpsertGatewayConfig(ctx, billing.GatewayConfig{
		TenantID: tenantID, Provider: "midtrans", Config: map[string]string{"server_key": serverKey}, IsActive: true,
	})
	require.NoError(t, err)
	_, err = r.Billing.CreatePayment(ctx, billing.Payment{
		TenantID: tenantID, InvoiceID: &inv.ID, CustomerID: c.ID, AmountIDR: 166500,
		Method: billing.MethodGateway, GatewayProvider: "midtrans", GatewayRef: orderID,
		Status: billing.PayPending, IdempotencyKey: orderID,
	})
	require.NoError(t, err)

	restorer := &fakeRestorer{}
	notifier := &fakeNotifier{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := paymentsvc.New(r, store, map[payment.Provider]payment.Gateway{
		payment.ProviderMidtrans: payment.NewMidtrans(nil),
	}, restorer, notifier, log)
	return svc, store, restorer, notifier
}

func TestHandleCallback_SettlesIdempotently(t *testing.T) {
	svc, store, restorer, notifier := setup(t)
	ctx := context.Background()
	raw := midtransBody(orderID, "200", "166500.00", "settlement", serverKey)

	require.NoError(t, svc.HandleCallback(ctx, payment.ProviderMidtrans, raw, http.Header{}))

	r := store.Repositories()
	pay, err := r.Billing.GetPaymentByRef(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, billing.PaySettled, pay.Status)

	inv, err := r.Billing.GetInvoice(ctx, tenantID, *pay.InvoiceID)
	require.NoError(t, err)
	assert.Equal(t, billing.StatusPaid, inv.Status)
	assert.Equal(t, []string{"budi|plan_1"}, restorer.calls)
	assert.Len(t, notifier.jobs, 1)

	income, _, err := r.Billing.SumLedger(ctx, tenantID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(166500), income)

	// Replaying the same callback must not double-book income.
	require.NoError(t, svc.HandleCallback(ctx, payment.ProviderMidtrans, raw, http.Header{}))
	income2, _, err := r.Billing.SumLedger(ctx, tenantID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(166500), income2, "idempotent: income booked once")
}

func TestHandleCallback_RejectsBadSignature(t *testing.T) {
	svc, _, _, _ := setup(t)
	raw := midtransBody(orderID, "200", "166500.00", "settlement", "WRONG-KEY")

	err := svc.HandleCallback(context.Background(), payment.ProviderMidtrans, raw, http.Header{})
	assert.True(t, paymentsvc.IsUnverified(err), "expected unverified error, got %v", err)
}

func TestHandleCallback_UnknownPayment(t *testing.T) {
	svc, _, _, _ := setup(t)
	raw := midtransBody("nonexistent-order", "200", "1000.00", "settlement", serverKey)

	err := svc.HandleCallback(context.Background(), payment.ProviderMidtrans, raw, http.Header{})
	assert.ErrorIs(t, err, billing.ErrPaymentNotFound)
}
