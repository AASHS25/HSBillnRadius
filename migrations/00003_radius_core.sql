-- +goose Up
-- FreeRADIUS-compatible tables, extended with tenant_id and indexes. The
-- radius-service reads these (or their cache); the billing side writes them
-- when provisioning customers and plans.

CREATE TABLE nas (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    nasname     TEXT        NOT NULL, -- IP or identifier
    shortname   TEXT        NOT NULL DEFAULT '',
    type        TEXT        NOT NULL DEFAULT 'other',
    ports       INT         NULL,
    secret      TEXT        NOT NULL,
    server      TEXT        NOT NULL DEFAULT '',
    community   TEXT        NOT NULL DEFAULT '',
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX nas_tenant_nasname_key ON nas (tenant_id, nasname);

CREATE TABLE radcheck (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    username  TEXT   NOT NULL,
    attribute TEXT   NOT NULL,
    op        TEXT   NOT NULL DEFAULT ':=',
    value     TEXT   NOT NULL
);
CREATE INDEX radcheck_tenant_username_idx ON radcheck (tenant_id, username);

CREATE TABLE radreply (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    username  TEXT   NOT NULL,
    attribute TEXT   NOT NULL,
    op        TEXT   NOT NULL DEFAULT ':=',
    value     TEXT   NOT NULL
);
CREATE INDEX radreply_tenant_username_idx ON radreply (tenant_id, username);

CREATE TABLE radusergroup (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    username  TEXT   NOT NULL,
    groupname TEXT   NOT NULL,
    priority  INT    NOT NULL DEFAULT 1
);
CREATE INDEX radusergroup_tenant_username_idx ON radusergroup (tenant_id, username);

CREATE TABLE radgroupcheck (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    groupname TEXT   NOT NULL,
    attribute TEXT   NOT NULL,
    op        TEXT   NOT NULL DEFAULT ':=',
    value     TEXT   NOT NULL
);
CREATE INDEX radgroupcheck_tenant_group_idx ON radgroupcheck (tenant_id, groupname);

CREATE TABLE radgroupreply (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    groupname TEXT   NOT NULL,
    attribute TEXT   NOT NULL,
    op        TEXT   NOT NULL DEFAULT ':=',
    value     TEXT   NOT NULL
);
CREATE INDEX radgroupreply_tenant_group_idx ON radgroupreply (tenant_id, groupname);

-- +goose Down
DROP TABLE IF EXISTS radgroupreply;
DROP TABLE IF EXISTS radgroupcheck;
DROP TABLE IF EXISTS radusergroup;
DROP TABLE IF EXISTS radreply;
DROP TABLE IF EXISTS radcheck;
DROP TABLE IF EXISTS nas;
