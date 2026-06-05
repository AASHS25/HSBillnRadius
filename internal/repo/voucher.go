package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/voucher"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type voucherRepo struct{ q *sqlc.Queries }

func toDomainBatch(m sqlc.VoucherBatch) voucher.Batch {
	return voucher.Batch{
		ID: m.ID, TenantID: m.TenantID, PlanID: m.PlanID, Prefix: m.Prefix, Qty: m.Qty,
		PriceIDR: m.PriceIdr, TemplateID: int8Ptr(m.TemplateID), CreatedBy: int8Ptr(m.CreatedBy), CreatedAt: m.CreatedAt,
	}
}

func toDomainVoucher(m sqlc.Voucher) voucher.Voucher {
	return voucher.Voucher{
		ID: m.ID, TenantID: m.TenantID, BatchID: m.BatchID, Code: m.Code, Username: m.Username,
		Password: m.Password, Status: voucher.Status(m.Status), UsedAt: tsPtr(m.UsedAt), SoldAt: tsPtr(m.SoldAt), CreatedAt: m.CreatedAt,
	}
}

func (r *voucherRepo) CreateBatch(ctx context.Context, b voucher.Batch) (voucher.Batch, error) {
	m, err := r.q.CreateVoucherBatch(ctx, sqlc.CreateVoucherBatchParams{
		TenantID: b.TenantID, PlanID: b.PlanID, Prefix: b.Prefix, Qty: b.Qty,
		PriceIdr: b.PriceIDR, TemplateID: pgInt8Ptr(b.TemplateID), CreatedBy: pgInt8Ptr(b.CreatedBy),
	})
	if err != nil {
		return voucher.Batch{}, fmt.Errorf("create voucher batch: %w", err)
	}
	return toDomainBatch(m), nil
}

func (r *voucherRepo) CreateVoucher(ctx context.Context, v voucher.Voucher) (voucher.Voucher, error) {
	m, err := r.q.CreateVoucher(ctx, sqlc.CreateVoucherParams{
		TenantID: v.TenantID, BatchID: v.BatchID, Code: v.Code, Username: v.Username, Password: v.Password,
	})
	if err != nil {
		return voucher.Voucher{}, fmt.Errorf("create voucher: %w", err)
	}
	return toDomainVoucher(m), nil
}

func (r *voucherRepo) GetBatch(ctx context.Context, tenantID, id int64) (voucher.Batch, error) {
	m, err := r.q.GetVoucherBatch(ctx, sqlc.GetVoucherBatchParams{ID: id, TenantID: tenantID})
	if err != nil {
		if isNotFound(err) {
			return voucher.Batch{}, voucher.ErrBatchNotFound
		}
		return voucher.Batch{}, fmt.Errorf("get voucher batch: %w", err)
	}
	return toDomainBatch(m), nil
}

func (r *voucherRepo) ListBatches(ctx context.Context, tenantID int64, limit, offset int32) ([]voucher.Batch, error) {
	rows, err := r.q.ListVoucherBatches(ctx, sqlc.ListVoucherBatchesParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list voucher batches: %w", err)
	}
	out := make([]voucher.Batch, len(rows))
	for i, m := range rows {
		out[i] = toDomainBatch(m)
	}
	return out, nil
}

func (r *voucherRepo) ListByBatch(ctx context.Context, tenantID, batchID int64, limit, offset int32) ([]voucher.Voucher, error) {
	rows, err := r.q.ListVouchersByBatch(ctx, sqlc.ListVouchersByBatchParams{BatchID: batchID, TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list vouchers: %w", err)
	}
	out := make([]voucher.Voucher, len(rows))
	for i, m := range rows {
		out[i] = toDomainVoucher(m)
	}
	return out, nil
}

func (r *voucherRepo) GetByCode(ctx context.Context, tenantID int64, code string) (voucher.Voucher, error) {
	m, err := r.q.GetVoucherByCode(ctx, sqlc.GetVoucherByCodeParams{TenantID: tenantID, Code: code})
	if err != nil {
		if isNotFound(err) {
			return voucher.Voucher{}, voucher.ErrNotFound
		}
		return voucher.Voucher{}, fmt.Errorf("get voucher: %w", err)
	}
	return toDomainVoucher(m), nil
}

func (r *voucherRepo) MarkUsed(ctx context.Context, tenantID int64, code string) error {
	if err := r.q.MarkVoucherUsed(ctx, sqlc.MarkVoucherUsedParams{TenantID: tenantID, Code: code}); err != nil {
		return fmt.Errorf("mark voucher used: %w", err)
	}
	return nil
}
