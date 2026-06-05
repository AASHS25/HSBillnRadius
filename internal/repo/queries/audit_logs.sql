-- name: InsertAuditLog :one
INSERT INTO audit_logs (tenant_id, actor_user_id, action, entity, entity_id, diff, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
