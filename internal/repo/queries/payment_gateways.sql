-- name: UpsertPaymentGateway :one
INSERT INTO payment_gateways (tenant_id, provider, config, is_active, is_production)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, provider) DO UPDATE SET
    config = EXCLUDED.config, is_active = EXCLUDED.is_active,
    is_production = EXCLUDED.is_production, updated_at = now()
RETURNING *;

-- name: GetPaymentGateway :one
SELECT * FROM payment_gateways
WHERE tenant_id = $1 AND provider = $2 AND is_active = true;

-- name: GetPaymentByGatewayRef :one
SELECT * FROM payments WHERE gateway_ref = $1;

-- name: SettlePayment :exec
UPDATE payments SET status = 'settled', paid_at = now(), raw_callback = $2
WHERE id = $1 AND status <> 'settled';

-- name: MarkPaymentStatus :exec
UPDATE payments SET status = $2, raw_callback = $3 WHERE id = $1;

-- name: ListPendingGatewayPayments :many
SELECT * FROM payments
WHERE status = 'pending' AND method = 'gateway' AND created_at < $1
ORDER BY created_at LIMIT $2;
