-- name: CreateRole :one
INSERT INTO roles (tenant_id, name, is_system)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByName :one
SELECT * FROM roles WHERE tenant_id = $1 AND name = $2;

-- name: ListRolesByTenant :many
SELECT * FROM roles WHERE tenant_id = $1 ORDER BY name;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GrantAllPermissionsToRole :exec
INSERT INTO role_permissions (role_id, permission_id)
SELECT $1, id FROM permissions
ON CONFLICT DO NOTHING;

-- name: ListPermissionCodesByRole :many
SELECT p.code
FROM role_permissions rp
JOIN permissions p ON p.id = rp.permission_id
WHERE rp.role_id = $1
ORDER BY p.code;
