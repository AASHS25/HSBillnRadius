-- +goose Up

-- Enums ----------------------------------------------------------------------
CREATE TYPE tenant_status AS ENUM ('trial', 'active', 'suspended', 'closed');

-- Tenants --------------------------------------------------------------------
CREATE TABLE tenants (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT          NOT NULL,
    slug       TEXT          NOT NULL,
    domain     TEXT          NULL,
    branding   JSONB         NOT NULL DEFAULT '{}'::jsonb,
    settings   JSONB         NOT NULL DEFAULT '{}'::jsonb,
    status     tenant_status NOT NULL DEFAULT 'trial',
    plan       TEXT          NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ   NULL
);
CREATE UNIQUE INDEX tenants_slug_key ON tenants (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_domain_key ON tenants (domain) WHERE domain IS NOT NULL AND deleted_at IS NULL;

-- Roles ----------------------------------------------------------------------
CREATE TABLE roles (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    is_system  BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX roles_tenant_name_key ON roles (tenant_id, name);

-- Permissions (global catalog) -----------------------------------------------
CREATE TABLE permissions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
CREATE INDEX role_permissions_permission_idx ON role_permissions (permission_id);

-- Users ----------------------------------------------------------------------
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    email         CITEXT      NOT NULL,
    password_hash TEXT        NOT NULL,
    role_id       BIGINT      NOT NULL REFERENCES roles (id),
    parent_id     BIGINT      NULL REFERENCES users (id),
    balance_idr   BIGINT      NOT NULL DEFAULT 0,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX users_tenant_email_key ON users (tenant_id, email) WHERE deleted_at IS NULL;
CREATE INDEX users_role_idx ON users (role_id);
CREATE INDEX users_parent_idx ON users (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX users_tenant_active_idx ON users (tenant_id, is_active);

-- Refresh tokens (rotating, revocable) ---------------------------------------
CREATE TABLE refresh_tokens (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    user_agent TEXT        NOT NULL DEFAULT '',
    ip         TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);

-- Audit log ------------------------------------------------------------------
CREATE TABLE audit_logs (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT      NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    actor_user_id BIGINT      NULL REFERENCES users (id) ON DELETE SET NULL,
    action        TEXT        NOT NULL,
    entity        TEXT        NOT NULL,
    entity_id     TEXT        NOT NULL DEFAULT '',
    diff          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    ip            TEXT        NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_tenant_created_idx ON audit_logs (tenant_id, created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id);

-- Baseline permission catalog ------------------------------------------------
INSERT INTO permissions (code, description) VALUES
    ('tenant.read',     'View tenant settings'),
    ('tenant.update',   'Update tenant settings'),
    ('user.create',     'Create users'),
    ('user.read',       'View users'),
    ('user.update',     'Update users'),
    ('user.delete',     'Delete users'),
    ('role.manage',     'Manage roles and permissions'),
    ('customer.create', 'Create customers'),
    ('customer.read',   'View customers'),
    ('customer.update', 'Update customers'),
    ('customer.delete', 'Delete customers'),
    ('plan.manage',     'Manage service plans'),
    ('invoice.read',    'View invoices'),
    ('invoice.manage',  'Create and modify invoices'),
    ('payment.read',    'View payments'),
    ('payment.manage',  'Record and verify payments')
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS tenants;
DROP TYPE IF EXISTS tenant_status;
