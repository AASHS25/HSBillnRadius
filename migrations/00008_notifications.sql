-- +goose Up

CREATE TYPE wa_provider AS ENUM ('fonnte', 'wablas', 'starsender', 'onesender', 'unofficial');
CREATE TYPE notif_channel AS ENUM ('wa', 'email');
CREATE TYPE notif_status AS ENUM ('queued', 'sent', 'failed');

CREATE TABLE wa_gateways (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    provider    wa_provider NOT NULL,
    config      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    daily_limit INT         NOT NULL DEFAULT 1000,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX wa_gateways_tenant_active_idx ON wa_gateways (tenant_id, is_active);

CREATE TABLE message_templates (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    key       TEXT          NOT NULL,
    channel   notif_channel NOT NULL DEFAULT 'wa',
    body      TEXT          NOT NULL,
    is_active BOOLEAN       NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX message_templates_tenant_key_chan ON message_templates (tenant_id, key, channel);

-- notification_logs doubles as the transactional job queue: rows start queued
-- with a next_attempt_at, workers claim due rows with FOR UPDATE SKIP LOCKED.
CREATE TABLE notification_logs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    channel         notif_channel NOT NULL DEFAULT 'wa',
    to_addr         TEXT          NOT NULL,
    template_key    TEXT          NOT NULL,
    payload         JSONB         NOT NULL DEFAULT '{}'::jsonb,
    status          notif_status  NOT NULL DEFAULT 'queued',
    provider_ref    TEXT          NOT NULL DEFAULT '',
    error           TEXT          NOT NULL DEFAULT '',
    attempts        INT           NOT NULL DEFAULT 0,
    dedup_key       TEXT          NOT NULL,
    next_attempt_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ   NULL
);
CREATE UNIQUE INDEX notification_logs_dedup_key ON notification_logs (dedup_key);
CREATE INDEX notification_logs_due_idx ON notification_logs (status, next_attempt_at)
    WHERE status = 'queued';
CREATE INDEX notification_logs_tenant_idx ON notification_logs (tenant_id, created_at DESC);

CREATE TABLE broadcasts (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    segment       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    template_body TEXT        NOT NULL,
    total         INT         NOT NULL DEFAULT 0,
    sent          INT         NOT NULL DEFAULT 0,
    status        TEXT        NOT NULL DEFAULT 'queued',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX broadcasts_tenant_idx ON broadcasts (tenant_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS broadcasts;
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS message_templates;
DROP TABLE IF EXISTS wa_gateways;
DROP TYPE IF EXISTS notif_status;
DROP TYPE IF EXISTS notif_channel;
DROP TYPE IF EXISTS wa_provider;
