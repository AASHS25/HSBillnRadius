-- +goose Up
CREATE TABLE deposits (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT         NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id     BIGINT         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount_idr  BIGINT         NOT NULL,
    method      payment_method NOT NULL DEFAULT 'transfer',
    gateway_ref TEXT           NULL,
    status      payment_status NOT NULL DEFAULT 'settled',
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX deposits_tenant_user_idx ON deposits (tenant_id, user_id, created_at DESC);

CREATE TABLE commissions (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id        BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    reseller_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_payment_id BIGINT     NULL REFERENCES payments (id) ON DELETE SET NULL,
    amount_idr       BIGINT      NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'settled',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX commissions_tenant_reseller_idx ON commissions (tenant_id, reseller_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS commissions;
DROP TABLE IF EXISTS deposits;
