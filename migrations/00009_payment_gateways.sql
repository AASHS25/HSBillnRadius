-- +goose Up
CREATE TYPE pg_provider AS ENUM ('midtrans', 'xendit', 'duitku', 'tripay');

CREATE TABLE payment_gateways (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    provider      pg_provider NOT NULL,
    config        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    is_production BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payment_gateways_tenant_provider ON payment_gateways (tenant_id, provider);
CREATE INDEX payment_gateways_tenant_active_idx ON payment_gateways (tenant_id, is_active);

-- +goose Down
DROP TABLE IF EXISTS payment_gateways;
DROP TYPE IF EXISTS pg_provider;
