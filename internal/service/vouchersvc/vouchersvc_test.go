package vouchersvc_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	"github.com/aashs25/hsbillnradius/internal/domain/voucher"
	"github.com/aashs25/hsbillnradius/internal/repo/memrepo"
	"github.com/aashs25/hsbillnradius/internal/service/vouchersvc"
)

func TestGenerateBatch_ProvisionsRadius(t *testing.T) {
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()
	p, err := r.Plan.Create(ctx, plan.Plan{TenantID: 1, Name: "Hotspot 1h", ServiceType: plan.ServiceHotspot})
	require.NoError(t, err)

	svc := vouchersvc.New(r, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	batch, vouchers, err := svc.GenerateBatch(ctx, 1, 5, vouchersvc.GenerateInput{
		PlanID: p.ID, Prefix: "WIFI", Qty: 3, PriceIDR: 5000,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 3, batch.Qty)
	require.Len(t, vouchers, 3)

	for _, v := range vouchers {
		assert.True(t, strings.HasPrefix(v.Code, "WIFI"))
		assert.Equal(t, voucher.StatusUnused, v.Status)

		pw, ok := store.RadCheckValue(1, v.Code, "Cleartext-Password")
		require.True(t, ok, "voucher must be in radcheck")
		assert.Equal(t, v.Password, pw)

		group, ok := store.UserGroupName(1, v.Code)
		require.True(t, ok)
		assert.Equal(t, p.GroupName(), group)
	}
}

func TestGenerateBatch_InvalidQty(t *testing.T) {
	store := memrepo.New()
	svc := vouchersvc.New(store.Repositories(), store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, _, err := svc.GenerateBatch(context.Background(), 1, 0, vouchersvc.GenerateInput{PlanID: 1, Qty: 0})
	assert.ErrorIs(t, err, voucher.ErrInvalidQty)
}

func TestRedeem(t *testing.T) {
	store := memrepo.New()
	r := store.Repositories()
	ctx := context.Background()
	p, _ := r.Plan.Create(ctx, plan.Plan{TenantID: 1, Name: "H", ServiceType: plan.ServiceHotspot})
	svc := vouchersvc.New(r, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, vouchers, err := svc.GenerateBatch(ctx, 1, 0, vouchersvc.GenerateInput{PlanID: p.ID, Qty: 1})
	require.NoError(t, err)

	v, err := svc.Redeem(ctx, 1, vouchers[0].Code)
	require.NoError(t, err)
	assert.Equal(t, voucher.StatusUsed, v.Status)
}
