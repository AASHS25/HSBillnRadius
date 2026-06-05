// Package resellersvc implements reseller balance top-ups and commission
// recording. Top-up credits the reseller's balance; commissions are recorded
// (and credited) when one of the reseller's customers pays.
package resellersvc

import (
	"context"
	"log/slog"

	"github.com/aashs25/hsbillnradius/internal/domain/reseller"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Service provides reseller operations.
type Service struct {
	repos repo.Repositories
	tx    repo.TxManager
	log   *slog.Logger
}

// New builds a reseller Service.
func New(repos repo.Repositories, tx repo.TxManager, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, log: log}
}

// Topup credits a reseller's balance with a deposit, atomically.
func (s *Service) Topup(ctx context.Context, tenantID, resellerID, amount int64, method string) (reseller.Deposit, int64, error) {
	if amount <= 0 {
		return reseller.Deposit{}, 0, reseller.ErrInvalidAmount
	}
	var dep reseller.Deposit
	var balance int64
	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		var err error
		dep, err = r.Reseller.CreateDeposit(ctx, reseller.Deposit{
			TenantID: tenantID, UserID: resellerID, AmountIDR: amount, Method: method, Status: "settled",
		})
		if err != nil {
			return err
		}
		balance, err = r.Reseller.AddBalance(ctx, resellerID, amount)
		return err
	})
	return dep, balance, err
}

// Record creates a commission and credits the reseller's balance. It satisfies
// billingsvc.Commissioner.
func (s *Service) Record(ctx context.Context, tenantID, resellerID, sourcePaymentID, amountIDR int64) error {
	if amountIDR <= 0 {
		return nil
	}
	return s.tx.WithTx(ctx, func(r repo.Repositories) error {
		src := &sourcePaymentID
		if sourcePaymentID == 0 {
			src = nil
		}
		if _, err := r.Reseller.CreateCommission(ctx, reseller.Commission{
			TenantID: tenantID, ResellerID: resellerID, SourcePaymentID: src, AmountIDR: amountIDR, Status: "settled",
		}); err != nil {
			return err
		}
		_, err := r.Reseller.AddBalance(ctx, resellerID, amountIDR)
		return err
	})
}

// ListDeposits returns a reseller's deposits.
func (s *Service) ListDeposits(ctx context.Context, tenantID, userID int64, limit, offset int32) ([]reseller.Deposit, error) {
	return s.repos.Reseller.ListDeposits(ctx, tenantID, userID, limit, offset)
}

// ListCommissions returns a reseller's commissions.
func (s *Service) ListCommissions(ctx context.Context, tenantID, resellerID int64, limit, offset int32) ([]reseller.Commission, error) {
	return s.repos.Reseller.ListCommissions(ctx, tenantID, resellerID, limit, offset)
}
