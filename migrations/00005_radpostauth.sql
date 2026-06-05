-- +goose Up
CREATE TABLE radpostauth (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id    BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    username     TEXT        NOT NULL,
    pass         TEXT        NOT NULL DEFAULT '',
    reply        TEXT        NOT NULL DEFAULT '',
    authdate     TIMESTAMPTZ NOT NULL DEFAULT now(),
    nasipaddress TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX radpostauth_tenant_user_idx ON radpostauth (tenant_id, username, authdate DESC);

-- +goose Down
DROP TABLE IF EXISTS radpostauth;
