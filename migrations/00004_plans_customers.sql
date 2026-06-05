-- +goose Up

CREATE TYPE service_type AS ENUM ('pppoe', 'hotspot', 'dhcp');
CREATE TYPE billing_cycle AS ENUM ('monthly', 'fixed', 'profile', 'prepaid_topup');
CREATE TYPE customer_status AS ENUM ('new', 'active', 'isolated', 'suspended', 'terminated', 'free');

-- Plans ----------------------------------------------------------------------
-- Note: tax is stored as integer basis points (1% = 100 bps) rather than
-- NUMERIC(5,2) so all money math stays in integers (no float for money).
CREATE TABLE plans (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id      BIGINT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name           TEXT          NOT NULL,
    service_type   service_type  NOT NULL DEFAULT 'pppoe',
    price_idr      BIGINT        NOT NULL DEFAULT 0,
    tax_bps        INT           NOT NULL DEFAULT 0,
    billing_cycle  billing_cycle NOT NULL DEFAULT 'monthly',
    active_days    INT           NOT NULL DEFAULT 30,
    data_quota_mb  BIGINT        NULL,
    time_quota_sec BIGINT        NULL,
    is_unlimited   BOOLEAN       NOT NULL DEFAULT true,
    pool_name      TEXT          NOT NULL DEFAULT '',
    isolir_plan_id BIGINT        NULL REFERENCES plans (id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ   NULL
);
CREATE INDEX plans_tenant_idx ON plans (tenant_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX plans_tenant_name_key ON plans (tenant_id, name) WHERE deleted_at IS NULL;

-- Bandwidth profiles (one per plan) ------------------------------------------
CREATE TABLE bandwidth_profiles (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id           BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    plan_id             BIGINT      NOT NULL REFERENCES plans (id) ON DELETE CASCADE,
    rate_limit_rx       TEXT        NOT NULL DEFAULT '',
    rate_limit_tx       TEXT        NOT NULL DEFAULT '',
    burst_rx            TEXT        NOT NULL DEFAULT '',
    burst_tx            TEXT        NOT NULL DEFAULT '',
    burst_threshold_rx  TEXT        NOT NULL DEFAULT '',
    burst_threshold_tx  TEXT        NOT NULL DEFAULT '',
    burst_time          TEXT        NOT NULL DEFAULT '',
    priority            INT         NOT NULL DEFAULT 8,
    mikrotik_rate_string TEXT       NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX bandwidth_profiles_plan_key ON bandwidth_profiles (plan_id);

-- Customers ------------------------------------------------------------------
-- pppoe_username/password hold the PPPoE service credentials that map to
-- radcheck/radusergroup (not in the original spec's column list, but required
-- to provision AAA for a PPPoE customer).
CREATE TABLE customers (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id      BIGINT          NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    customer_no    TEXT            NOT NULL,
    name           TEXT            NOT NULL,
    id_card_no     TEXT            NOT NULL DEFAULT '',
    email          TEXT            NOT NULL DEFAULT '',
    phone_wa       TEXT            NOT NULL DEFAULT '',
    address        TEXT            NOT NULL DEFAULT '',
    lat            DOUBLE PRECISION NULL,
    lng            DOUBLE PRECISION NULL,
    install_date   DATE            NULL,
    status         customer_status NOT NULL DEFAULT 'new',
    plan_id        BIGINT          NULL REFERENCES plans (id),
    reseller_id    BIGINT          NULL REFERENCES users (id),
    balance_idr    BIGINT          NOT NULL DEFAULT 0,
    pppoe_username TEXT            NULL,
    pppoe_password TEXT            NULL,
    notes          TEXT            NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ     NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ     NULL
);
CREATE UNIQUE INDEX customers_tenant_no_key ON customers (tenant_id, customer_no) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX customers_tenant_pppoe_key ON customers (tenant_id, pppoe_username) WHERE pppoe_username IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX customers_tenant_status_idx ON customers (tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX customers_plan_idx ON customers (plan_id);
CREATE INDEX customers_reseller_idx ON customers (reseller_id) WHERE reseller_id IS NOT NULL;

CREATE TABLE customer_documents (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id BIGINT      NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    type        TEXT        NOT NULL,
    file_path   TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX customer_documents_customer_idx ON customer_documents (customer_id);

-- +goose Down
DROP TABLE IF EXISTS customer_documents;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS bandwidth_profiles;
DROP TABLE IF EXISTS plans;
DROP TYPE IF EXISTS customer_status;
DROP TYPE IF EXISTS billing_cycle;
DROP TYPE IF EXISTS service_type;
