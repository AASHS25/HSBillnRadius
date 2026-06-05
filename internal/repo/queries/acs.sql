-- name: UpsertAcsDevice :one
INSERT INTO acs_devices (tenant_id, customer_id, device_id, serial_number, manufacturer, model, wan_ip, ssid, last_inform_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (tenant_id, device_id) DO UPDATE SET
    customer_id = EXCLUDED.customer_id, serial_number = EXCLUDED.serial_number,
    manufacturer = EXCLUDED.manufacturer, model = EXCLUDED.model, wan_ip = EXCLUDED.wan_ip,
    ssid = EXCLUDED.ssid, last_inform_at = now(), updated_at = now()
RETURNING *;

-- name: ListAcsDevices :many
SELECT * FROM acs_devices WHERE tenant_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3;

-- name: GetAcsDevice :one
SELECT * FROM acs_devices WHERE tenant_id = $1 AND device_id = $2;
