-- name: CreateVoucherBatch :one
INSERT INTO voucher_batches (tenant_id, plan_id, prefix, qty, price_idr, template_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateVoucher :one
INSERT INTO vouchers (tenant_id, batch_id, code, username, password, status)
VALUES ($1, $2, $3, $4, $5, 'unused')
RETURNING *;

-- name: GetVoucherBatch :one
SELECT * FROM voucher_batches WHERE id = $1 AND tenant_id = $2;

-- name: ListVoucherBatches :many
SELECT * FROM voucher_batches WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: ListVouchersByBatch :many
SELECT * FROM vouchers WHERE batch_id = $1 AND tenant_id = $2 ORDER BY id LIMIT $3 OFFSET $4;

-- name: GetVoucherByCode :one
SELECT * FROM vouchers WHERE tenant_id = $1 AND code = $2;

-- name: MarkVoucherUsed :exec
UPDATE vouchers SET status = 'used', used_at = now()
WHERE tenant_id = $1 AND code = $2 AND status = 'unused';

-- name: CountVouchersByStatus :one
SELECT count(*) FROM vouchers WHERE tenant_id = $1 AND status = $2;
