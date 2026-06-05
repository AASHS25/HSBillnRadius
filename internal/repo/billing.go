package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aashs25/hsbillnradius/internal/domain/billing"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type billingRepo struct{ q *sqlc.Queries }

func toDomainInvoice(m sqlc.Invoice) billing.Invoice {
	return billing.Invoice{
		ID:          m.ID,
		TenantID:    m.TenantID,
		InvoiceNo:   m.InvoiceNo,
		CustomerID:  m.CustomerID,
		PeriodStart: dateVal(m.PeriodStart),
		PeriodEnd:   dateVal(m.PeriodEnd),
		SubtotalIDR: m.SubtotalIdr,
		TaxIDR:      m.TaxIdr,
		DiscountIDR: m.DiscountIdr,
		TotalIDR:    m.TotalIdr,
		DueDate:     dateVal(m.DueDate),
		Status:      billing.Status(m.Status),
		Type:        billing.Type(m.Type),
		IssuedAt:    tsPtr(m.IssuedAt),
		PaidAt:      tsPtr(m.PaidAt),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func toDomainItem(m sqlc.InvoiceItem) billing.InvoiceItem {
	return billing.InvoiceItem{
		ID:           m.ID,
		InvoiceID:    m.InvoiceID,
		Description:  m.Description,
		Qty:          m.Qty,
		UnitPriceIDR: m.UnitPriceIdr,
		AmountIDR:    m.AmountIdr,
		Type:         billing.ItemType(m.Type),
	}
}

func toDomainPayment(m sqlc.Payment) billing.Payment {
	return billing.Payment{
		ID:               m.ID,
		TenantID:         m.TenantID,
		InvoiceID:        int8Ptr(m.InvoiceID),
		CustomerID:       m.CustomerID,
		AmountIDR:        m.AmountIdr,
		Method:           billing.PaymentMethod(m.Method),
		GatewayProvider:  m.GatewayProvider,
		GatewayRef:       textVal(m.GatewayRef),
		Status:           billing.PaymentStatus(m.Status),
		PaidAt:           tsPtr(m.PaidAt),
		VerifiedByUserID: int8Ptr(m.VerifiedByUserID),
		IdempotencyKey:   m.IdempotencyKey,
		RawCallback:      m.RawCallback,
		CreatedAt:        m.CreatedAt,
	}
}

func (r *billingRepo) CreateInvoice(ctx context.Context, inv billing.Invoice) (billing.Invoice, error) {
	m, err := r.q.CreateInvoice(ctx, sqlc.CreateInvoiceParams{
		TenantID:    inv.TenantID,
		InvoiceNo:   inv.InvoiceNo,
		CustomerID:  inv.CustomerID,
		PeriodStart: pgDate(inv.PeriodStart),
		PeriodEnd:   pgDate(inv.PeriodEnd),
		SubtotalIdr: inv.SubtotalIDR,
		TaxIdr:      inv.TaxIDR,
		DiscountIdr: inv.DiscountIDR,
		TotalIdr:    inv.TotalIDR,
		DueDate:     pgDate(inv.DueDate),
		Status:      sqlc.InvoiceStatus(inv.Status),
		Type:        sqlc.InvoiceType(inv.Type),
	})
	if err != nil {
		if isNotFound(err) { // ON CONFLICT DO NOTHING returned no row
			return billing.Invoice{}, billing.ErrInvoiceExists
		}
		return billing.Invoice{}, fmt.Errorf("create invoice: %w", err)
	}
	return toDomainInvoice(m), nil
}

func (r *billingRepo) AddInvoiceItem(ctx context.Context, it billing.InvoiceItem) (billing.InvoiceItem, error) {
	m, err := r.q.InsertInvoiceItem(ctx, sqlc.InsertInvoiceItemParams{
		InvoiceID:    it.InvoiceID,
		Description:  it.Description,
		Qty:          it.Qty,
		UnitPriceIdr: it.UnitPriceIDR,
		AmountIdr:    it.AmountIDR,
		Type:         sqlc.InvoiceItemType(it.Type),
	})
	if err != nil {
		return billing.InvoiceItem{}, fmt.Errorf("insert invoice item: %w", err)
	}
	return toDomainItem(m), nil
}

func (r *billingRepo) GetInvoice(ctx context.Context, tenantID, id int64) (billing.Invoice, error) {
	m, err := r.q.GetInvoiceByID(ctx, sqlc.GetInvoiceByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		if isNotFound(err) {
			return billing.Invoice{}, billing.ErrInvoiceNotFound
		}
		return billing.Invoice{}, fmt.Errorf("get invoice: %w", err)
	}
	return toDomainInvoice(m), nil
}

func (r *billingRepo) InvoiceItems(ctx context.Context, invoiceID int64) ([]billing.InvoiceItem, error) {
	rows, err := r.q.ListInvoiceItems(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list invoice items: %w", err)
	}
	items := make([]billing.InvoiceItem, len(rows))
	for i, m := range rows {
		items[i] = toDomainItem(m)
	}
	return items, nil
}

func (r *billingRepo) ListInvoices(ctx context.Context, tenantID int64, limit, offset int32) ([]billing.Invoice, error) {
	rows, err := r.q.ListInvoicesByTenant(ctx, sqlc.ListInvoicesByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	out := make([]billing.Invoice, len(rows))
	for i, m := range rows {
		out[i] = toDomainInvoice(m)
	}
	return out, nil
}

func (r *billingRepo) SetInvoiceStatus(ctx context.Context, tenantID, id int64, status billing.Status) error {
	if err := r.q.SetInvoiceStatus(ctx, sqlc.SetInvoiceStatusParams{ID: id, TenantID: tenantID, Status: sqlc.InvoiceStatus(status)}); err != nil {
		return fmt.Errorf("set invoice status: %w", err)
	}
	return nil
}

func (r *billingRepo) MarkInvoicePaid(ctx context.Context, tenantID, id int64) error {
	if err := r.q.MarkInvoicePaid(ctx, sqlc.MarkInvoicePaidParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("mark invoice paid: %w", err)
	}
	return nil
}

func (r *billingRepo) MarkOverdue(ctx context.Context) (int64, error) {
	n, err := r.q.MarkOverdueInvoices(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark overdue invoices: %w", err)
	}
	return n, nil
}

func (r *billingRepo) CreatePayment(ctx context.Context, p billing.Payment) (billing.Payment, error) {
	m, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		TenantID:         p.TenantID,
		InvoiceID:        pgInt8Ptr(p.InvoiceID),
		CustomerID:       p.CustomerID,
		AmountIdr:        p.AmountIDR,
		Method:           sqlc.PaymentMethod(p.Method),
		GatewayProvider:  p.GatewayProvider,
		GatewayRef:       pgTextOrNull(p.GatewayRef),
		Status:           sqlc.PaymentStatus(p.Status),
		PaidAt:           pgTimestamptzPtr(p.PaidAt),
		VerifiedByUserID: pgInt8Ptr(p.VerifiedByUserID),
		IdempotencyKey:   p.IdempotencyKey,
		RawCallback:      jsonOrEmpty(p.RawCallback),
	})
	if err != nil {
		if isNotFound(err) {
			return billing.Payment{}, billing.ErrPaymentNotUnique
		}
		return billing.Payment{}, fmt.Errorf("create payment: %w", err)
	}
	return toDomainPayment(m), nil
}

func (r *billingRepo) ListPayments(ctx context.Context, tenantID int64, limit, offset int32) ([]billing.Payment, error) {
	rows, err := r.q.ListPaymentsByTenant(ctx, sqlc.ListPaymentsByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	out := make([]billing.Payment, len(rows))
	for i, m := range rows {
		out[i] = toDomainPayment(m)
	}
	return out, nil
}

func (r *billingRepo) AddLedger(ctx context.Context, e billing.LedgerEntry) (billing.LedgerEntry, error) {
	m, err := r.q.InsertLedgerEntry(ctx, sqlc.InsertLedgerEntryParams{
		TenantID:    e.TenantID,
		Type:        sqlc.LedgerType(e.Type),
		Category:    e.Category,
		AmountIdr:   e.AmountIDR,
		RefType:     e.RefType,
		RefID:       pgInt8Ptr(e.RefID),
		Description: e.Description,
		CreatedBy:   pgInt8Ptr(e.CreatedBy),
	})
	if err != nil {
		return billing.LedgerEntry{}, fmt.Errorf("insert ledger entry: %w", err)
	}
	return billing.LedgerEntry{
		ID: m.ID, TenantID: m.TenantID, Type: billing.LedgerType(m.Type), Category: m.Category,
		AmountIDR: m.AmountIdr, RefType: m.RefType, RefID: int8Ptr(m.RefID),
		Description: m.Description, CreatedBy: int8Ptr(m.CreatedBy), CreatedAt: m.CreatedAt,
	}, nil
}

func (r *billingRepo) SumLedger(ctx context.Context, tenantID int64, from, to time.Time) (income, expense int64, err error) {
	rows, err := r.q.SumLedgerByType(ctx, sqlc.SumLedgerByTypeParams{
		TenantID:    tenantID,
		CreatedAt:   from,
		CreatedAt_2: to,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("sum ledger: %w", err)
	}
	for _, row := range rows {
		switch billing.LedgerType(row.Type) {
		case billing.LedgerIncome:
			income = row.Total
		case billing.LedgerExpense:
			expense = row.Total
		}
	}
	return income, expense, nil
}

func (r *billingRepo) Outstanding(ctx context.Context, tenantID int64) (int64, error) {
	n, err := r.q.SumOutstanding(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("sum outstanding: %w", err)
	}
	return n, nil
}

func (r *billingRepo) GetPaymentByRef(ctx context.Context, ref string) (billing.Payment, error) {
	m, err := r.q.GetPaymentByGatewayRef(ctx, pgTextOrNull(ref))
	if err != nil {
		if isNotFound(err) {
			return billing.Payment{}, billing.ErrPaymentNotFound
		}
		return billing.Payment{}, fmt.Errorf("get payment by ref: %w", err)
	}
	return toDomainPayment(m), nil
}

func (r *billingRepo) SettlePayment(ctx context.Context, id int64, raw []byte) error {
	if err := r.q.SettlePayment(ctx, sqlc.SettlePaymentParams{ID: id, RawCallback: jsonOrEmpty(raw)}); err != nil {
		return fmt.Errorf("settle payment: %w", err)
	}
	return nil
}

func (r *billingRepo) MarkPaymentStatus(ctx context.Context, id int64, status billing.PaymentStatus, raw []byte) error {
	if err := r.q.MarkPaymentStatus(ctx, sqlc.MarkPaymentStatusParams{
		ID: id, Status: sqlc.PaymentStatus(status), RawCallback: jsonOrEmpty(raw),
	}); err != nil {
		return fmt.Errorf("mark payment status: %w", err)
	}
	return nil
}

func (r *billingRepo) ListPendingGatewayPayments(ctx context.Context, olderThan time.Time, limit int32) ([]billing.Payment, error) {
	rows, err := r.q.ListPendingGatewayPayments(ctx, sqlc.ListPendingGatewayPaymentsParams{CreatedAt: olderThan, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list pending gateway payments: %w", err)
	}
	out := make([]billing.Payment, len(rows))
	for i, m := range rows {
		out[i] = toDomainPayment(m)
	}
	return out, nil
}

func (r *billingRepo) GetGatewayConfig(ctx context.Context, tenantID int64, provider string) (billing.GatewayConfig, error) {
	m, err := r.q.GetPaymentGateway(ctx, sqlc.GetPaymentGatewayParams{TenantID: tenantID, Provider: sqlc.PgProvider(provider)})
	if err != nil {
		if isNotFound(err) {
			return billing.GatewayConfig{}, billing.ErrGatewayNotEnabled
		}
		return billing.GatewayConfig{}, fmt.Errorf("get gateway config: %w", err)
	}
	config := map[string]string{}
	_ = json.Unmarshal(m.Config, &config)
	return billing.GatewayConfig{
		ID: m.ID, TenantID: m.TenantID, Provider: string(m.Provider), Config: config,
		IsActive: m.IsActive, IsProduction: m.IsProduction,
	}, nil
}

func (r *billingRepo) UpsertGatewayConfig(ctx context.Context, cfg billing.GatewayConfig) (billing.GatewayConfig, error) {
	config, _ := json.Marshal(cfg.Config)
	m, err := r.q.UpsertPaymentGateway(ctx, sqlc.UpsertPaymentGatewayParams{
		TenantID: cfg.TenantID, Provider: sqlc.PgProvider(cfg.Provider), Config: config,
		IsActive: cfg.IsActive, IsProduction: cfg.IsProduction,
	})
	if err != nil {
		return billing.GatewayConfig{}, fmt.Errorf("upsert gateway config: %w", err)
	}
	cfg.ID = m.ID
	return cfg, nil
}
