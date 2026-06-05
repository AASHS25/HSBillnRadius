-- name: CreateTenant :one
INSERT INTO tenants (name, slug, domain, branding, settings, status, plan)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1 AND deleted_at IS NULL;

-- name: GetTenantByDomain :one
SELECT * FROM tenants WHERE domain = $1 AND deleted_at IS NULL;

-- name: UpdateTenant :one
UPDATE tenants
SET name = $2, domain = $3, branding = $4, settings = $5, status = $6, plan = $7, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;
