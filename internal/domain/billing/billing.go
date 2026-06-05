// Package billing holds invoice/payment/ledger entities and the pure billing
// rules: proration, money totals (all int64 rupiah), and the invoice status
// machine. No external dependencies, no floats for money.
package billing

import (
	"errors"
	"time"
)

// Status is an invoice lifecycle state.
type Status string

const (
	StatusDraft   Status = "draft"
	StatusUnpaid  Status = "unpaid"
	StatusPaid    Status = "paid"
	StatusOverdue Status = "overdue"
	StatusVoid    Status = "void"
)

// Type distinguishes prepaid from postpaid invoices.
type Type string

const (
	TypePrepaid  Type = "prepaid"
	TypePostpaid Type = "postpaid"
)

// ItemType categorizes an invoice line.
type ItemType string

const (
	ItemPlan    ItemType = "plan"
	ItemAddon   ItemType = "addon"
	ItemInstall ItemType = "install"
	ItemDevice  ItemType = "device"
)

// PaymentMethod is how a payment was made.
type PaymentMethod string

const (
	MethodCash     PaymentMethod = "cash"
	MethodTransfer PaymentMethod = "transfer"
	MethodGateway  PaymentMethod = "gateway"
	MethodBalance  PaymentMethod = "balance"
)

// PaymentStatus is a payment lifecycle state.
type PaymentStatus string

const (
	PayPending PaymentStatus = "pending"
	PaySettled PaymentStatus = "settled"
	PayFailed  PaymentStatus = "failed"
	PayExpired PaymentStatus = "expired"
)

// LedgerType is income or expense.
type LedgerType string

const (
	LedgerIncome  LedgerType = "income"
	LedgerExpense LedgerType = "expense"
)

// Domain errors.
var (
	ErrInvoiceNotFound    = errors.New("invoice not found")
	ErrInvoiceExists      = errors.New("invoice already exists for period")
	ErrInvalidStatus      = errors.New("invalid invoice status transition")
	ErrAlreadyPaid        = errors.New("invoice already paid")
	ErrPaymentNotUnique   = errors.New("duplicate payment idempotency key")
	ErrAmountMismatch     = errors.New("payment amount does not cover invoice total")
	ErrPaymentNotFound    = errors.New("payment not found")
	ErrGatewayNotEnabled  = errors.New("payment gateway not configured")
	ErrCallbackUnverified = errors.New("payment callback signature invalid")
)

// GatewayConfig is a tenant's payment-gateway credentials.
type GatewayConfig struct {
	ID           int64
	TenantID     int64
	Provider     string
	Config       map[string]string
	IsActive     bool
	IsProduction bool
}

// Invoice is a customer bill.
type Invoice struct {
	ID          int64
	TenantID    int64
	InvoiceNo   string
	CustomerID  int64
	PeriodStart time.Time
	PeriodEnd   time.Time
	SubtotalIDR int64
	TaxIDR      int64
	DiscountIDR int64
	TotalIDR    int64
	DueDate     time.Time
	Status      Status
	Type        Type
	IssuedAt    *time.Time
	PaidAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Items       []InvoiceItem
}

// InvoiceItem is a single billed line.
type InvoiceItem struct {
	ID           int64
	InvoiceID    int64
	Description  string
	Qty          int32
	UnitPriceIDR int64
	AmountIDR    int64
	Type         ItemType
}

// Payment records money received against an invoice or customer.
type Payment struct {
	ID               int64
	TenantID         int64
	InvoiceID        *int64
	CustomerID       int64
	AmountIDR        int64
	Method           PaymentMethod
	GatewayProvider  string
	GatewayRef       string
	Status           PaymentStatus
	PaidAt           *time.Time
	VerifiedByUserID *int64
	IdempotencyKey   string
	RawCallback      []byte
	CreatedAt        time.Time
}

// LedgerEntry is a cashbook income/expense record.
type LedgerEntry struct {
	ID          int64
	TenantID    int64
	Type        LedgerType
	Category    string
	AmountIDR   int64
	RefType     string
	RefID       *int64
	Description string
	CreatedBy   *int64
	CreatedAt   time.Time
}

// Totals is the computed money breakdown of an invoice.
type Totals struct {
	Subtotal int64
	Discount int64
	Tax      int64
	Total    int64
}

// ComputeTotals derives tax and total from a subtotal, applying the discount
// before tax. Tax is in basis points (1% = 100 bps). All integer math.
func ComputeTotals(subtotalIDR, discountIDR int64, taxBps int32) Totals {
	taxable := subtotalIDR - discountIDR
	if taxable < 0 {
		taxable = 0
	}
	tax := taxable * int64(taxBps) / 10000
	return Totals{
		Subtotal: subtotalIDR,
		Discount: discountIDR,
		Tax:      tax,
		Total:    taxable + tax,
	}
}

// Prorate returns the proportional price for the part of a billing cycle that
// remains from `from` until cycleEnd. Charging from before the start yields the
// full price; from on/after the end yields zero. Integer (floor) math.
func Prorate(priceIDR int64, cycleStart, cycleEnd, from time.Time) int64 {
	total := dayCount(cycleStart, cycleEnd)
	if total <= 0 {
		return priceIDR
	}
	if !from.After(cycleStart) {
		return priceIDR
	}
	if !from.Before(cycleEnd) {
		return 0
	}
	remaining := dayCount(from, cycleEnd)
	return priceIDR * int64(remaining) / int64(total)
}

// dayCount returns whole days between two instants (truncated to the day).
func dayCount(a, b time.Time) int {
	d := truncateDay(b).Sub(truncateDay(a))
	return int(d.Hours() / 24)
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// transitions defines the allowed invoice status moves.
var transitions = map[Status]map[Status]bool{
	StatusDraft:   {StatusUnpaid: true, StatusVoid: true},
	StatusUnpaid:  {StatusPaid: true, StatusOverdue: true, StatusVoid: true},
	StatusOverdue: {StatusPaid: true, StatusVoid: true},
	StatusPaid:    {},
	StatusVoid:    {},
}

// CanTransition reports whether moving from->to is allowed.
func CanTransition(from, to Status) bool {
	return transitions[from][to]
}
