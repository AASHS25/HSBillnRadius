-- name: CreatePlan :one
INSERT INTO plans (
    tenant_id, name, service_type, price_idr, tax_bps, billing_cycle,
    active_days, data_quota_mb, time_quota_sec, is_unlimited, pool_name, isolir_plan_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetPlanByID :one
SELECT * FROM plans WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ListPlansByTenant :many
SELECT * FROM plans
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: CountPlansByTenant :one
SELECT count(*) FROM plans WHERE tenant_id = $1 AND deleted_at IS NULL;

-- name: UpdatePlan :one
UPDATE plans SET
    name = $3, service_type = $4, price_idr = $5, tax_bps = $6, billing_cycle = $7,
    active_days = $8, data_quota_mb = $9, time_quota_sec = $10, is_unlimited = $11,
    pool_name = $12, isolir_plan_id = $13, updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePlan :exec
UPDATE plans SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;
