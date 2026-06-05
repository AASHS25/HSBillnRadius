-- name: CreateCustomer :one
INSERT INTO customers (
    tenant_id, customer_no, name, id_card_no, email, phone_wa, address,
    lat, lng, install_date, status, plan_id, reseller_id, balance_idr,
    pppoe_username, pppoe_password, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetCustomerByID :one
SELECT * FROM customers WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ListCustomersByTenant :many
SELECT * FROM customers
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCustomersByTenant :one
SELECT count(*) FROM customers WHERE tenant_id = $1 AND deleted_at IS NULL;

-- name: UpdateCustomer :one
UPDATE customers SET
    name = $3, id_card_no = $4, email = $5, phone_wa = $6, address = $7,
    lat = $8, lng = $9, install_date = $10, status = $11, plan_id = $12,
    reseller_id = $13, pppoe_username = $14, pppoe_password = $15, notes = $16,
    updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateCustomerStatus :exec
UPDATE customers SET status = $3, updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: SoftDeleteCustomer :exec
UPDATE customers SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: SetCustomerActiveUntil :exec
UPDATE customers SET active_until = $3, status = $4, updated_at = now()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ListExpiredActiveCustomers :many
SELECT id, tenant_id, pppoe_username, plan_id, status
FROM customers
WHERE deleted_at IS NULL AND status = 'active'
  AND active_until IS NOT NULL AND active_until < now()
ORDER BY active_until
LIMIT $1;
