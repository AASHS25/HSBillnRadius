-- +goose Up
CREATE TYPE voucher_status AS ENUM ('unused', 'used', 'expired', 'disabled');

CREATE TABLE voucher_templates (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    html       TEXT        NOT NULL DEFAULT '',
    is_default BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX voucher_templates_tenant_idx ON voucher_templates (tenant_id);

CREATE TABLE voucher_batches (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    plan_id     BIGINT      NOT NULL REFERENCES plans (id),
    prefix      TEXT        NOT NULL DEFAULT '',
    qty         INT         NOT NULL,
    price_idr   BIGINT      NOT NULL DEFAULT 0,
    template_id BIGINT      NULL REFERENCES voucher_templates (id),
    created_by  BIGINT      NULL REFERENCES users (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX voucher_batches_tenant_idx ON voucher_batches (tenant_id, created_at DESC);

CREATE TABLE vouchers (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT         NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    batch_id   BIGINT         NOT NULL REFERENCES voucher_batches (id) ON DELETE CASCADE,
    code       TEXT           NOT NULL,
    username   TEXT           NOT NULL,
    password   TEXT           NOT NULL,
    status     voucher_status NOT NULL DEFAULT 'unused',
    used_at    TIMESTAMPTZ    NULL,
    sold_at    TIMESTAMPTZ    NULL,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX vouchers_tenant_code_key ON vouchers (tenant_id, code);
CREATE INDEX vouchers_tenant_status_idx ON vouchers (tenant_id, status);
CREATE INDEX vouchers_batch_idx ON vouchers (batch_id);

-- +goose Down
DROP TABLE IF EXISTS vouchers;
DROP TABLE IF EXISTS voucher_batches;
DROP TABLE IF EXISTS voucher_templates;
DROP TYPE IF EXISTS voucher_status;
