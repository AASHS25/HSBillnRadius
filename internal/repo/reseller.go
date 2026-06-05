package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/reseller"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type resellerRepo struct{ q *sqlc.Queries }

func (r *resellerRepo) CreateDeposit(ctx context.Context, d reseller.Deposit) (reseller.Deposit, error) {
	m, err := r.q.CreateDeposit(ctx, sqlc.CreateDepositParams{
		TenantID: d.TenantID, UserID: d.UserID, AmountIdr: d.AmountIDR,
		Method: sqlc.PaymentMethod(d.Method), GatewayRef: pgTextOrNull(d.GatewayRef),
		Status: sqlc.PaymentStatus(statusOr(d.Status, "settled")),
	})
	if err != nil {
		return reseller.Deposit{}, fmt.Errorf("create deposit: %w", err)
	}
	return reseller.Deposit{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, AmountIDR: m.AmountIdr,
		Method: string(m.Method), GatewayRef: textVal(m.GatewayRef), Status: string(m.Status), CreatedAt: m.CreatedAt,
	}, nil
}

func (r *resellerRepo) AddBalance(ctx context.Context, userID, delta int64) (int64, error) {
	balance, err := r.q.AddUserBalance(ctx, sqlc.AddUserBalanceParams{ID: userID, BalanceIdr: delta})
	if err != nil {
		return 0, fmt.Errorf("add user balance: %w", err)
	}
	return balance, nil
}

func (r *resellerRepo) CreateCommission(ctx context.Context, c reseller.Commission) (reseller.Commission, error) {
	m, err := r.q.CreateCommission(ctx, sqlc.CreateCommissionParams{
		TenantID: c.TenantID, ResellerID: c.ResellerID, SourcePaymentID: pgInt8Ptr(c.SourcePaymentID), AmountIdr: c.AmountIDR,
	})
	if err != nil {
		return reseller.Commission{}, fmt.Errorf("create commission: %w", err)
	}
	return reseller.Commission{
		ID: m.ID, TenantID: m.TenantID, ResellerID: m.ResellerID, SourcePaymentID: int8Ptr(m.SourcePaymentID),
		AmountIDR: m.AmountIdr, Status: m.Status, CreatedAt: m.CreatedAt,
	}, nil
}

func (r *resellerRepo) ListDeposits(ctx context.Context, tenantID, userID int64, limit, offset int32) ([]reseller.Deposit, error) {
	rows, err := r.q.ListDeposits(ctx, sqlc.ListDepositsParams{TenantID: tenantID, UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list deposits: %w", err)
	}
	out := make([]reseller.Deposit, len(rows))
	for i, m := range rows {
		out[i] = reseller.Deposit{
			ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, AmountIDR: m.AmountIdr,
			Method: string(m.Method), GatewayRef: textVal(m.GatewayRef), Status: string(m.Status), CreatedAt: m.CreatedAt,
		}
	}
	return out, nil
}

func (r *resellerRepo) ListCommissions(ctx context.Context, tenantID, resellerID int64, limit, offset int32) ([]reseller.Commission, error) {
	rows, err := r.q.ListCommissions(ctx, sqlc.ListCommissionsParams{TenantID: tenantID, ResellerID: resellerID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list commissions: %w", err)
	}
	out := make([]reseller.Commission, len(rows))
	for i, m := range rows {
		out[i] = reseller.Commission{
			ID: m.ID, TenantID: m.TenantID, ResellerID: m.ResellerID, SourcePaymentID: int8Ptr(m.SourcePaymentID),
			AmountIDR: m.AmountIdr, Status: m.Status, CreatedAt: m.CreatedAt,
		}
	}
	return out, nil
}

func statusOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
