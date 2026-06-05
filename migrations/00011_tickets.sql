-- +goose Up
CREATE TYPE ticket_type AS ENUM ('trouble', 'install', 'other');
CREATE TYPE ticket_status AS ENUM ('open', 'in_progress', 'resolved', 'closed');

CREATE TABLE tickets (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id        BIGINT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    customer_id      BIGINT        NULL REFERENCES customers (id) ON DELETE SET NULL,
    type             ticket_type   NOT NULL DEFAULT 'trouble',
    subject          TEXT          NOT NULL,
    description      TEXT          NOT NULL DEFAULT '',
    status           ticket_status NOT NULL DEFAULT 'open',
    priority         INT           NOT NULL DEFAULT 3,
    assigned_user_id BIGINT        NULL REFERENCES users (id),
    created_by       BIGINT        NULL REFERENCES users (id),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ   NULL
);
CREATE INDEX tickets_tenant_status_idx ON tickets (tenant_id, status, created_at DESC);
CREATE INDEX tickets_customer_idx ON tickets (customer_id);
CREATE INDEX tickets_assigned_idx ON tickets (assigned_user_id) WHERE assigned_user_id IS NOT NULL;

CREATE TABLE ticket_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id   BIGINT      NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    user_id     BIGINT      NULL REFERENCES users (id),
    note        TEXT        NOT NULL DEFAULT '',
    status_from TEXT        NOT NULL DEFAULT '',
    status_to   TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_events_ticket_idx ON ticket_events (ticket_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS ticket_events;
DROP TABLE IF EXISTS tickets;
DROP TYPE IF EXISTS ticket_status;
DROP TYPE IF EXISTS ticket_type;
