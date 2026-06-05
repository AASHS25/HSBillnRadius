-- +goose Up

CREATE TYPE invoice_status AS ENUM ('draft', 'unpaid', 'paid', 'overdue', 'void');
CREATE TYPE invoice_type AS ENUM ('prepaid', 'postpaid');
CREATE TYPE invoice_item_type AS ENUM ('plan', 'addon', 'install', 'device');
CREATE TYPE payment_method AS ENUM ('cash', 'transfer', 'gateway', 'balance');
CREATE TYPE payment_status AS ENUM ('pending', 'settled', 'failed', 'expired');
CREATE TYPE ledger_type AS ENUM ('income', 'expense');

-- Customer service expiry, extended on payment; auto-isolir scans this.
ALTER TABLE customers ADD COLUMN active_until TIMESTAMPTZ NULL;
CREATE INDEX customers_active_until_idx ON customers (tenant_id, active_until)
    WHERE deleted_at IS NULL;

CREATE TABLE invoices (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id    BIGINT         NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    invoice_no   TEXT           NOT NULL,
    customer_id  BIGINT         NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    period_start DATE           NOT NULL,
    period_end   DATE           NOT NULL,
    subtotal_idr BIGINT         NOT NULL DEFAULT 0,
    tax_idr      BIGINT         NOT NULL DEFAULT 0,
    discount_idr BIGINT         NOT NULL DEFAULT 0,
    total_idr    BIGINT         NOT NULL DEFAULT 0,
    due_date     DATE           NOT NULL,
    status       invoice_status NOT NULL DEFAULT 'unpaid',
    type         invoice_type   NOT NULL DEFAULT 'postpaid',
    issued_at    TIMESTAMPTZ    NULL,
    paid_at      TIMESTAMPTZ    NULL,
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX invoices_tenant_no_key ON invoices (tenant_id, invoice_no);
CREATE UNIQUE INDEX invoices_customer_period_key ON invoices (customer_id, period_start);
CREATE INDEX invoices_tenant_status_due_idx ON invoices (tenant_id, status, due_date);

CREATE TABLE invoice_items (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id     BIGINT            NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    description    TEXT              NOT NULL,
    qty            INT               NOT NULL DEFAULT 1,
    unit_price_idr BIGINT            NOT NULL DEFAULT 0,
    amount_idr     BIGINT            NOT NULL DEFAULT 0,
    type           invoice_item_type NOT NULL DEFAULT 'plan'
);
CREATE INDEX invoice_items_invoice_idx ON invoice_items (invoice_id);

CREATE TABLE payments (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id           BIGINT         NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    invoice_id          BIGINT         NULL REFERENCES invoices (id) ON DELETE SET NULL,
    customer_id         BIGINT         NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    amount_idr          BIGINT         NOT NULL,
    method              payment_method NOT NULL,
    gateway_provider    TEXT           NOT NULL DEFAULT '',
    gateway_ref         TEXT           NULL,
    status              payment_status NOT NULL DEFAULT 'pending',
    paid_at             TIMESTAMPTZ    NULL,
    verified_by_user_id BIGINT         NULL REFERENCES users (id),
    idempotency_key     TEXT           NOT NULL,
    raw_callback        JSONB          NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payments_idempotency_key ON payments (idempotency_key);
CREATE UNIQUE INDEX payments_gateway_ref_key ON payments (gateway_ref) WHERE gateway_ref IS NOT NULL;
CREATE INDEX payments_tenant_customer_idx ON payments (tenant_id, customer_id);

CREATE TABLE ledger_entries (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    type        ledger_type NOT NULL,
    category    TEXT        NOT NULL DEFAULT '',
    amount_idr  BIGINT      NOT NULL,
    ref_type    TEXT        NOT NULL DEFAULT '',
    ref_id      BIGINT      NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_by  BIGINT      NULL REFERENCES users (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ledger_tenant_created_idx ON ledger_entries (tenant_id, created_at DESC);
CREATE INDEX ledger_tenant_type_idx ON ledger_entries (tenant_id, type);

-- +goose Down
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS invoice_items;
DROP TABLE IF EXISTS invoices;
ALTER TABLE customers DROP COLUMN IF EXISTS active_until;
DROP TYPE IF EXISTS ledger_type;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS invoice_item_type;
DROP TYPE IF EXISTS invoice_type;
DROP TYPE IF EXISTS invoice_status;
