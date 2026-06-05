-- name: CreateInvoice :one
-- Idempotent per (customer, period): a conflict returns no row, which the repo
-- maps to ErrInvoiceExists.
INSERT INTO invoices (
    tenant_id, invoice_no, customer_id, period_start, period_end,
    subtotal_idr, tax_idr, discount_idr, total_idr, due_date, status, type, issued_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
ON CONFLICT (customer_id, period_start) DO NOTHING
RETURNING *;

-- name: InsertInvoiceItem :one
INSERT INTO invoice_items (invoice_id, description, qty, unit_price_idr, amount_idr, type)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = $1 AND tenant_id = $2;

-- name: ListInvoiceItems :many
SELECT * FROM invoice_items WHERE invoice_id = $1 ORDER BY id;

-- name: ListInvoicesByTenant :many
SELECT * FROM invoices WHERE tenant_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: ListInvoicesByCustomer :many
SELECT * FROM invoices WHERE tenant_id = $1 AND customer_id = $2
ORDER BY period_start DESC LIMIT $3 OFFSET $4;

-- name: SetInvoiceStatus :exec
UPDATE invoices SET status = $3, updated_at = now()
WHERE id = $1 AND tenant_id = $2;

-- name: MarkInvoicePaid :exec
UPDATE invoices SET status = 'paid', paid_at = now(), updated_at = now()
WHERE id = $1 AND tenant_id = $2;

-- name: MarkOverdueInvoices :execrows
UPDATE invoices SET status = 'overdue', updated_at = now()
WHERE status = 'unpaid' AND due_date < CURRENT_DATE;

-- name: CreatePayment :one
INSERT INTO payments (
    tenant_id, invoice_id, customer_id, amount_idr, method, gateway_provider,
    gateway_ref, status, paid_at, verified_by_user_id, idempotency_key, raw_callback
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING *;

-- name: ListPaymentsByTenant :many
SELECT * FROM payments WHERE tenant_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: InsertLedgerEntry :one
INSERT INTO ledger_entries (tenant_id, type, category, amount_idr, ref_type, ref_id, description, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: SumLedgerByType :many
SELECT type, COALESCE(SUM(amount_idr), 0)::BIGINT AS total
FROM ledger_entries
WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3
GROUP BY type;

-- name: SumOutstanding :one
SELECT COALESCE(SUM(total_idr), 0)::BIGINT
FROM invoices
WHERE tenant_id = $1 AND status IN ('unpaid', 'overdue');
