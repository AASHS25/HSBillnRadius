-- name: CreateDeposit :one
INSERT INTO deposits (tenant_id, user_id, amount_idr, method, gateway_ref, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDeposits :many
SELECT * FROM deposits WHERE tenant_id = $1 AND user_id = $2
ORDER BY created_at DESC LIMIT $3 OFFSET $4;

-- name: AddUserBalance :one
UPDATE users SET balance_idr = balance_idr + $2, updated_at = now()
WHERE id = $1
RETURNING balance_idr;

-- name: CreateCommission :one
INSERT INTO commissions (tenant_id, reseller_id, source_payment_id, amount_idr, status)
VALUES ($1, $2, $3, $4, 'settled')
RETURNING *;

-- name: ListCommissions :many
SELECT * FROM commissions WHERE tenant_id = $1 AND reseller_id = $2
ORDER BY created_at DESC LIMIT $3 OFFSET $4;
