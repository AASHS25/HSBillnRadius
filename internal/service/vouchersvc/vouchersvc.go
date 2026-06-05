// Package vouchersvc generates voucher batches. Each voucher is provisioned as
// a RADIUS hotspot credential (radcheck + radusergroup -> plan group) so it can
// be used to log in at the hotspot immediately.
package vouchersvc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aashs25/hsbillnradius/internal/domain/voucher"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

const codeLength = 6

// Service provides voucher operations.
type Service struct {
	repos repo.Repositories
	tx    repo.TxManager
	log   *slog.Logger
}

// New builds a voucher Service.
func New(repos repo.Repositories, tx repo.TxManager, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, log: log}
}

// GenerateInput is a batch-generation request.
type GenerateInput struct {
	PlanID   int64
	Prefix   string
	Qty      int32
	PriceIDR int64
}

// GenerateBatch creates a batch and qty vouchers, provisioning each into RADIUS.
func (s *Service) GenerateBatch(ctx context.Context, tenantID, actorID int64, in GenerateInput) (voucher.Batch, []voucher.Voucher, error) {
	if err := voucher.ValidateQty(in.Qty); err != nil {
		return voucher.Batch{}, nil, err
	}
	p, err := s.repos.Plan.GetByID(ctx, tenantID, in.PlanID)
	if err != nil {
		return voucher.Batch{}, nil, err
	}
	group := p.GroupName()

	var batch voucher.Batch
	var vouchers []voucher.Voucher
	err = s.tx.WithTx(ctx, func(r repo.Repositories) error {
		var bErr error
		batch, bErr = r.Voucher.CreateBatch(ctx, voucher.Batch{
			TenantID: tenantID, PlanID: in.PlanID, Prefix: in.Prefix, Qty: in.Qty,
			PriceIDR: in.PriceIDR, CreatedBy: actorPtr(actorID),
		})
		if bErr != nil {
			return bErr
		}
		for i := int32(0); i < in.Qty; i++ {
			v, vErr := s.createOne(ctx, r, tenantID, batch.ID, in.Prefix, group)
			if vErr != nil {
				return vErr
			}
			vouchers = append(vouchers, v)
		}
		return nil
	})
	if err != nil {
		return voucher.Batch{}, nil, err
	}
	return batch, vouchers, nil
}

// createOne generates a unique code and provisions the voucher into RADIUS.
func (s *Service) createOne(ctx context.Context, r repo.Repositories, tenantID, batchID int64, prefix, group string) (voucher.Voucher, error) {
	const maxRetry = 4
	for attempt := 0; attempt < maxRetry; attempt++ {
		code, err := voucher.GenerateCode(prefix, codeLength)
		if err != nil {
			return voucher.Voucher{}, err
		}
		v, err := r.Voucher.CreateVoucher(ctx, voucher.Voucher{
			TenantID: tenantID, BatchID: batchID, Code: code, Username: code, Password: code,
		})
		if err != nil {
			continue // likely a code collision; retry with a new code
		}
		if err := r.RadiusMap.SetUserPassword(ctx, tenantID, code, code); err != nil {
			return voucher.Voucher{}, err
		}
		if err := r.RadiusMap.SetUserGroup(ctx, tenantID, code, group, 1); err != nil {
			return voucher.Voucher{}, err
		}
		return v, nil
	}
	return voucher.Voucher{}, fmt.Errorf("could not generate a unique voucher code after %d attempts", maxRetry)
}

// ListBatches returns a page of batches.
func (s *Service) ListBatches(ctx context.Context, tenantID int64, limit, offset int32) ([]voucher.Batch, error) {
	return s.repos.Voucher.ListBatches(ctx, tenantID, limit, offset)
}

// ListVouchers returns vouchers in a batch.
func (s *Service) ListVouchers(ctx context.Context, tenantID, batchID int64, limit, offset int32) ([]voucher.Voucher, error) {
	return s.repos.Voucher.ListByBatch(ctx, tenantID, batchID, limit, offset)
}

// Redeem marks a voucher used (idempotent for already-used codes).
func (s *Service) Redeem(ctx context.Context, tenantID int64, code string) (voucher.Voucher, error) {
	v, err := s.repos.Voucher.GetByCode(ctx, tenantID, code)
	if err != nil {
		return voucher.Voucher{}, err
	}
	if err := s.repos.Voucher.MarkUsed(ctx, tenantID, code); err != nil {
		return voucher.Voucher{}, err
	}
	v.Status = voucher.StatusUsed
	return v, nil
}

func actorPtr(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}
