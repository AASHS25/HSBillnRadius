// Package paymentsvc handles payment-gateway charges and webhook settlement.
// The webhook path verifies the provider signature, is idempotent, and settles
// the invoice (mark paid + ledger income + extend service + restore + notify)
// inside a single transaction.
package paymentsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/ports/payment"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// Restorer restores RADIUS service after payment (optional).
type Restorer interface {
	Restore(ctx context.Context, tenantID int64, username, planGroup string) error
}

// Notifier enqueues notifications (optional).
type Notifier interface {
	Enqueue(ctx context.Context, job notification.Job) error
}

// Service handles gateway charges and callbacks.
type Service struct {
	repos    repo.Repositories
	tx       repo.TxManager
	gateways map[payment.Provider]payment.Gateway
	restorer Restorer
	notifier Notifier
	log      *slog.Logger
	now      func() time.Time
}

// New builds a payment Service.
func New(repos repo.Repositories, tx repo.TxManager, gateways map[payment.Provider]payment.Gateway, restorer Restorer, notifier Notifier, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, gateways: gateways, restorer: restorer, notifier: notifier, log: log, now: time.Now}
}

// ConfigureGateway stores a tenant's gateway credentials.
func (s *Service) ConfigureGateway(ctx context.Context, cfg billing.GatewayConfig) (billing.GatewayConfig, error) {
	return s.repos.Billing.UpsertGatewayConfig(ctx, cfg)
}

// CreateCharge initiates a gateway charge for an unpaid invoice and stores a
// pending payment keyed by the generated order id.
func (s *Service) CreateCharge(ctx context.Context, tenantID, invoiceID int64, provider payment.Provider) (payment.ChargeResponse, error) {
	gw, ok := s.gateways[provider]
	if !ok {
		return payment.ChargeResponse{}, fmt.Errorf("%w: %s", billing.ErrGatewayNotEnabled, provider)
	}
	inv, err := s.repos.Billing.GetInvoice(ctx, tenantID, invoiceID)
	if err != nil {
		return payment.ChargeResponse{}, err
	}
	if inv.Status == billing.StatusPaid {
		return payment.ChargeResponse{}, billing.ErrAlreadyPaid
	}
	cfg, err := s.repos.Billing.GetGatewayConfig(ctx, tenantID, string(provider))
	if err != nil {
		return payment.ChargeResponse{}, err
	}
	cust, err := s.repos.Customer.GetByID(ctx, tenantID, inv.CustomerID)
	if err != nil {
		return payment.ChargeResponse{}, err
	}

	orderID := fmt.Sprintf("t%d-inv%d-%d", tenantID, invoiceID, s.now().Unix())
	resp, err := gw.CreateCharge(ctx, payment.ChargeRequest{
		OrderID:       orderID,
		AmountIDR:     inv.TotalIDR,
		CustomerName:  cust.Name,
		CustomerEmail: cust.Email,
		CustomerPhone: cust.PhoneWA,
		Config:        cfg.Config,
		IsProduction:  cfg.IsProduction,
	})
	if err != nil {
		return payment.ChargeResponse{}, err
	}

	if _, err := s.repos.Billing.CreatePayment(ctx, billing.Payment{
		TenantID:        tenantID,
		InvoiceID:       &inv.ID,
		CustomerID:      inv.CustomerID,
		AmountIDR:       inv.TotalIDR,
		Method:          billing.MethodGateway,
		GatewayProvider: string(provider),
		GatewayRef:      orderID,
		Status:          billing.PayPending,
		IdempotencyKey:  orderID,
		RawCallback:     resp.Raw,
	}); err != nil {
		return payment.ChargeResponse{}, err
	}
	return resp, nil
}

// HandleCallback processes a provider webhook: it locates the local payment,
// verifies the signature with the tenant's config, and settles idempotently.
func (s *Service) HandleCallback(ctx context.Context, provider payment.Provider, raw []byte, headers http.Header) error {
	gw, ok := s.gateways[provider]
	if !ok {
		return fmt.Errorf("%w: %s", billing.ErrGatewayNotEnabled, provider)
	}

	orderID, err := gw.ExtractOrderID(raw)
	if err != nil {
		return err
	}
	pay, err := s.repos.Billing.GetPaymentByRef(ctx, orderID)
	if err != nil {
		return err
	}
	cfg, err := s.repos.Billing.GetGatewayConfig(ctx, pay.TenantID, string(provider))
	if err != nil {
		return err
	}

	result, err := gw.VerifyCallback(ctx, raw, headers, cfg.Config)
	if err != nil {
		return err
	}
	if !result.Verified {
		return billing.ErrCallbackUnverified
	}

	// Idempotency: a settled payment is a no-op (return success).
	if pay.Status == billing.PaySettled {
		return nil
	}

	switch result.Status {
	case billing.PaySettled:
		return s.settle(ctx, pay, result.Raw)
	case billing.PayFailed, billing.PayExpired:
		return s.repos.Billing.MarkPaymentStatus(ctx, pay.ID, result.Status, result.Raw)
	default:
		return nil // still pending
	}
}

// settle marks the payment settled, the invoice paid, books income and extends
// the customer's service in one transaction, then restores RADIUS + notifies.
func (s *Service) settle(ctx context.Context, pay billing.Payment, raw []byte) error {
	var cust = struct {
		username  string
		planGroup string
		phone     string
		name      string
		hasPlan   bool
	}{}

	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		if err := r.Billing.SettlePayment(ctx, pay.ID, raw); err != nil {
			return err
		}
		if pay.InvoiceID == nil {
			return nil
		}
		inv, err := r.Billing.GetInvoice(ctx, pay.TenantID, *pay.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Status == billing.StatusPaid {
			return nil
		}
		if err := r.Billing.MarkInvoicePaid(ctx, pay.TenantID, inv.ID); err != nil {
			return err
		}
		if _, err := r.Billing.AddLedger(ctx, billing.LedgerEntry{
			TenantID: pay.TenantID, Type: billing.LedgerIncome, Category: "subscription",
			AmountIDR: pay.AmountIDR, RefType: "invoice", RefID: &inv.ID,
			Description: "Gateway payment for " + inv.InvoiceNo,
		}); err != nil {
			return err
		}

		c, err := r.Customer.GetByID(ctx, pay.TenantID, inv.CustomerID)
		if err != nil {
			return err
		}
		cust.username, cust.phone, cust.name = c.PppoeUsername, c.PhoneWA, c.Name
		until := s.now()
		if c.ActiveUntil != nil && c.ActiveUntil.After(until) {
			until = *c.ActiveUntil
		}
		days := 30
		if c.PlanID != nil {
			if p, pErr := r.Plan.GetByID(ctx, pay.TenantID, *c.PlanID); pErr == nil {
				cust.hasPlan, cust.planGroup = true, p.GroupName()
				if p.ActiveDays > 0 {
					days = int(p.ActiveDays)
				}
			}
		}
		return r.Customer.SetActiveUntil(ctx, pay.TenantID, c.ID, until.AddDate(0, 0, days), customer.StatusActive)
	})
	if err != nil {
		return err
	}

	if s.restorer != nil && cust.hasPlan && cust.username != "" {
		if rErr := s.restorer.Restore(ctx, pay.TenantID, cust.username, cust.planGroup); rErr != nil {
			s.log.WarnContext(ctx, "restore after gateway settle failed", slog.Any("error", rErr))
		}
	}
	if s.notifier != nil && cust.phone != "" {
		_ = s.notifier.Enqueue(ctx, notification.Job{
			TenantID: pay.TenantID, Channel: notification.ChannelWA, To: cust.phone,
			TemplateKey: "paid", Vars: map[string]string{"nama": cust.name},
			DedupKey: fmt.Sprintf("gw-paid-%s", pay.GatewayRef),
		})
	}
	return nil
}

// PollPending re-checks pending gateway payments older than the cutoff (webhook
// fallback). For brevity it only logs; status re-fetch is provider-specific.
func (s *Service) PollPending(ctx context.Context, olderThan time.Duration, limit int32) (int, error) {
	pending, err := s.repos.Billing.ListPendingGatewayPayments(ctx, s.now().Add(-olderThan), limit)
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}

// IsUnverified reports whether err is a signature-verification failure.
func IsUnverified(err error) bool { return errors.Is(err, billing.ErrCallbackUnverified) }
