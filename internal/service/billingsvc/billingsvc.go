// Package billingsvc implements billing use-cases: invoice generation (with
// proration), payment settlement (ledger + service extension + RADIUS restore),
// overdue marking and financial reports. All money is int64 rupiah.
package billingsvc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/audit"
	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/domain/customer"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/domain/plan"
	repo "github.com/aashs25/hsbillnradius/internal/ports/repo"
)

// dueDays is the default number of days a postpaid invoice is due in.
const dueDays = 7

// Restorer restores a customer's RADIUS service after they pay (optional).
type Restorer interface {
	Restore(ctx context.Context, tenantID int64, username, planGroup string) error
}

// Isolator isolates a customer's RADIUS service (auto-isolir).
type Isolator interface {
	Isolate(ctx context.Context, tenantID int64, username, isolirGroup string) error
}

// Notifier enqueues a notification (optional integration).
type Notifier interface {
	Enqueue(ctx context.Context, job notification.Job) error
}

// Commissioner records a reseller commission (optional integration).
type Commissioner interface {
	Record(ctx context.Context, tenantID, resellerID, sourcePaymentID, amountIDR int64) error
}

// Service provides billing operations.
type Service struct {
	repos         repo.Repositories
	tx            repo.TxManager
	restorer      Restorer
	notifier      Notifier
	commissioner  Commissioner
	commissionBps int32
	log           *slog.Logger
	now           func() time.Time
}

// New builds a billing Service. restorer may be nil (RADIUS restore skipped).
func New(repos repo.Repositories, tx repo.TxManager, restorer Restorer, log *slog.Logger) *Service {
	return &Service{repos: repos, tx: tx, restorer: restorer, log: log, now: time.Now}
}

// WithNotifier attaches a notifier so payments enqueue a "paid" message.
func (s *Service) WithNotifier(n Notifier) *Service {
	s.notifier = n
	return s
}

// WithCommission attaches reseller commissioning at the given rate (basis points).
func (s *Service) WithCommission(rateBps int32, c Commissioner) *Service {
	s.commissionBps = rateBps
	s.commissioner = c
	return s
}

// GenerateMonthly creates (idempotently) the invoice covering the month that
// contains periodStart, prorating the first bill from the install date.
func (s *Service) GenerateMonthly(ctx context.Context, tenantID, customerID, actorID int64, periodStart time.Time) (billing.Invoice, error) {
	cust, err := s.repos.Customer.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return billing.Invoice{}, err
	}
	if cust.PlanID == nil {
		return billing.Invoice{}, fmt.Errorf("%w: customer has no plan", customer.ErrPlanRequired)
	}
	p, err := s.repos.Plan.GetByID(ctx, tenantID, *cust.PlanID)
	if err != nil {
		return billing.Invoice{}, err
	}

	periodStart = startOfMonth(periodStart)
	periodEnd := periodStart.AddDate(0, 1, 0)

	chargeFrom := periodStart
	if cust.InstallDate != nil && cust.InstallDate.After(periodStart) {
		chargeFrom = *cust.InstallDate
	}
	amount := billing.Prorate(p.PriceIDR, periodStart, periodEnd, chargeFrom)
	totals := billing.ComputeTotals(amount, 0, p.TaxBps)

	inv := billing.Invoice{
		TenantID:    tenantID,
		InvoiceNo:   fmt.Sprintf("INV-%s-%d", periodStart.Format("200601"), customerID),
		CustomerID:  customerID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		SubtotalIDR: totals.Subtotal,
		TaxIDR:      totals.Tax,
		DiscountIDR: totals.Discount,
		TotalIDR:    totals.Total,
		DueDate:     periodStart.AddDate(0, 0, dueDays),
		Status:      billing.StatusUnpaid,
		Type:        billing.TypePostpaid,
	}

	var created billing.Invoice
	err = s.tx.WithTx(ctx, func(r repo.Repositories) error {
		created, err = r.Billing.CreateInvoice(ctx, inv)
		if err != nil {
			return err
		}
		_, err = r.Billing.AddInvoiceItem(ctx, billing.InvoiceItem{
			InvoiceID:    created.ID,
			Description:  p.Name,
			Qty:          1,
			UnitPriceIDR: amount,
			AmountIDR:    amount,
			Type:         billing.ItemPlan,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "invoice.generate", created.ID)
		return nil
	})
	if err != nil {
		return billing.Invoice{}, err
	}
	return created, nil
}

// PayInvoice settles an invoice: records the payment, marks it paid, books a
// ledger income, extends the customer's service and restores RADIUS.
func (s *Service) PayInvoice(ctx context.Context, tenantID, invoiceID, actorID int64, method billing.PaymentMethod) (billing.Payment, error) {
	var payment billing.Payment
	var cust customer.Customer
	var p plan.Plan
	var hasPlan bool

	err := s.tx.WithTx(ctx, func(r repo.Repositories) error {
		inv, err := r.Billing.GetInvoice(ctx, tenantID, invoiceID)
		if err != nil {
			return err
		}
		if inv.Status == billing.StatusPaid {
			return billing.ErrAlreadyPaid
		}

		now := s.now()
		actor := actorPtr(actorID)
		payment, err = r.Billing.CreatePayment(ctx, billing.Payment{
			TenantID:         tenantID,
			InvoiceID:        &inv.ID,
			CustomerID:       inv.CustomerID,
			AmountIDR:        inv.TotalIDR,
			Method:           method,
			Status:           billing.PaySettled,
			PaidAt:           &now,
			VerifiedByUserID: actor,
			IdempotencyKey:   fmt.Sprintf("manual-inv-%d", inv.ID),
		})
		if err != nil {
			return err
		}
		if err := r.Billing.MarkInvoicePaid(ctx, tenantID, inv.ID); err != nil {
			return err
		}
		if _, err := r.Billing.AddLedger(ctx, billing.LedgerEntry{
			TenantID:    tenantID,
			Type:        billing.LedgerIncome,
			Category:    "subscription",
			AmountIDR:   inv.TotalIDR,
			RefType:     "invoice",
			RefID:       &inv.ID,
			Description: "Payment for " + inv.InvoiceNo,
			CreatedBy:   actor,
		}); err != nil {
			return err
		}

		cust, err = r.Customer.GetByID(ctx, tenantID, inv.CustomerID)
		if err != nil {
			return err
		}
		if cust.PlanID != nil {
			if p, err = r.Plan.GetByID(ctx, tenantID, *cust.PlanID); err == nil {
				hasPlan = true
			}
		}

		until := s.extend(cust, p, hasPlan)
		if err := r.Customer.SetActiveUntil(ctx, tenantID, cust.ID, until, customer.StatusActive); err != nil {
			return err
		}
		s.audit(ctx, r, tenantID, actorID, "invoice.pay", inv.ID)
		return nil
	})
	if err != nil {
		return billing.Payment{}, err
	}

	// Restore RADIUS outside the transaction (best effort).
	if s.restorer != nil && hasPlan && cust.PppoeUsername != "" {
		if err := s.restorer.Restore(ctx, tenantID, cust.PppoeUsername, p.GroupName()); err != nil {
			s.log.WarnContext(ctx, "radius restore after payment failed", slog.Any("error", err))
		}
	}

	// Notify the customer that their payment was received (best effort).
	if s.notifier != nil && cust.PhoneWA != "" {
		if err := s.notifier.Enqueue(ctx, notification.Job{
			TenantID:    tenantID,
			Channel:     notification.ChannelWA,
			To:          cust.PhoneWA,
			TemplateKey: "paid",
			Vars: map[string]string{
				"nama":       cust.Name,
				"tagihan":    strconv.FormatInt(payment.AmountIDR, 10),
				"invoice_no": fmt.Sprintf("manual-inv-%d", invoiceID),
			},
			DedupKey: fmt.Sprintf("paid-inv-%d", invoiceID),
		}); err != nil {
			s.log.WarnContext(ctx, "enqueue paid notification failed", slog.Any("error", err))
		}
	}
	// Record reseller commission when the customer belongs to a reseller.
	if s.commissioner != nil && s.commissionBps > 0 && cust.ResellerID != nil {
		amount := payment.AmountIDR * int64(s.commissionBps) / 10000
		if err := s.commissioner.Record(ctx, tenantID, *cust.ResellerID, payment.ID, amount); err != nil {
			s.log.WarnContext(ctx, "record commission failed", slog.Any("error", err))
		}
	}
	return payment, nil
}

// extend returns the new active_until: current expiry (if in the future) or now,
// plus the plan's active days.
func (s *Service) extend(cust customer.Customer, p plan.Plan, hasPlan bool) time.Time {
	base := s.now()
	if cust.ActiveUntil != nil && cust.ActiveUntil.After(base) {
		base = *cust.ActiveUntil
	}
	days := 30
	if hasPlan && p.ActiveDays > 0 {
		days = int(p.ActiveDays)
	}
	return base.AddDate(0, 0, days)
}

// Get returns an invoice with its line items.
func (s *Service) Get(ctx context.Context, tenantID, invoiceID int64) (billing.Invoice, error) {
	inv, err := s.repos.Billing.GetInvoice(ctx, tenantID, invoiceID)
	if err != nil {
		return billing.Invoice{}, err
	}
	items, err := s.repos.Billing.InvoiceItems(ctx, inv.ID)
	if err != nil {
		return billing.Invoice{}, err
	}
	inv.Items = items
	return inv, nil
}

// List returns a page of invoices.
func (s *Service) List(ctx context.Context, tenantID int64, limit, offset int32) ([]billing.Invoice, error) {
	return s.repos.Billing.ListInvoices(ctx, tenantID, limit, offset)
}

// Void cancels an unpaid/overdue invoice.
func (s *Service) Void(ctx context.Context, tenantID, actorID, invoiceID int64) error {
	inv, err := s.repos.Billing.GetInvoice(ctx, tenantID, invoiceID)
	if err != nil {
		return err
	}
	if !billing.CanTransition(inv.Status, billing.StatusVoid) {
		return billing.ErrInvalidStatus
	}
	return s.repos.Billing.SetInvoiceStatus(ctx, tenantID, invoiceID, billing.StatusVoid)
}

// MarkOverdue flips unpaid invoices past their due date to overdue.
func (s *Service) MarkOverdue(ctx context.Context) (int64, error) {
	return s.repos.Billing.MarkOverdue(ctx)
}

// RunIsolirScan finds active customers whose service has expired and isolates
// them: switch RADIUS to the isolir group (CoA kick) and mark them isolated.
// Returns the number of customers isolated.
func (s *Service) RunIsolirScan(ctx context.Context, isolator Isolator, limit int32) (int, error) {
	expired, err := s.repos.Customer.ListExpiredActive(ctx, limit)
	if err != nil {
		return 0, err
	}
	var count int
	for _, c := range expired {
		group := "isolir"
		if c.PlanID != nil {
			if p, gErr := s.repos.Plan.GetByID(ctx, c.TenantID, *c.PlanID); gErr == nil && p.IsolirPlanID != nil {
				group = fmt.Sprintf("plan_%d", *p.IsolirPlanID)
			}
		}
		if c.PppoeUsername != "" {
			if err := isolator.Isolate(ctx, c.TenantID, c.PppoeUsername, group); err != nil {
				s.log.WarnContext(ctx, "auto-isolir failed", slog.Any("error", err), slog.Int64("customer", c.ID))
			}
		}
		if err := s.repos.Customer.UpdateStatus(ctx, c.TenantID, c.ID, customer.StatusIsolated); err != nil {
			s.log.WarnContext(ctx, "set isolated status failed", slog.Any("error", err), slog.Int64("customer", c.ID))
			continue
		}
		count++
	}
	return count, nil
}

// Summary is a financial report for a period.
type Summary struct {
	Income      int64 `json:"income"`
	Expense     int64 `json:"expense"`
	Net         int64 `json:"net"`
	Outstanding int64 `json:"outstanding"`
}

// Report aggregates ledger income/expense over [from,to) plus outstanding total.
func (s *Service) Report(ctx context.Context, tenantID int64, from, to time.Time) (Summary, error) {
	income, expense, err := s.repos.Billing.SumLedger(ctx, tenantID, from, to)
	if err != nil {
		return Summary{}, err
	}
	outstanding, err := s.repos.Billing.Outstanding(ctx, tenantID)
	if err != nil {
		return Summary{}, err
	}
	return Summary{Income: income, Expense: expense, Net: income - expense, Outstanding: outstanding}, nil
}

func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func actorPtr(actorID int64) *int64 {
	if actorID == 0 {
		return nil
	}
	return &actorID
}

func (s *Service) audit(ctx context.Context, r repo.Repositories, tenantID, actorID int64, action string, entityID int64) {
	if _, err := r.Audit.Insert(ctx, audit.Entry{
		TenantID:    tenantID,
		ActorUserID: actorPtr(actorID),
		Action:      action,
		Entity:      "invoice",
		EntityID:    strconv.FormatInt(entityID, 10),
	}); err != nil {
		s.log.WarnContext(ctx, "audit insert failed", slog.Any("error", err), slog.String("action", action))
	}
}

// ListByCustomer returns a customer's invoices (client area).
func (s *Service) ListByCustomer(ctx context.Context, tenantID, customerID int64, limit, offset int32) ([]billing.Invoice, error) {
	return s.repos.Billing.ListInvoicesByCustomer(ctx, tenantID, customerID, limit, offset)
}
