-- name: EnqueueNotification :execrows
INSERT INTO notification_logs (tenant_id, channel, to_addr, template_key, payload, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (dedup_key) DO NOTHING;

-- name: ClaimDueNotifications :many
-- Atomically lease up to $1 due rows (bump next_attempt_at to the lease and
-- increment attempts) so concurrent workers don't double-process.
UPDATE notification_logs SET next_attempt_at = $2, attempts = attempts + 1
WHERE id IN (
    SELECT id FROM notification_logs
    WHERE status = 'queued' AND next_attempt_at <= now()
    ORDER BY next_attempt_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, channel, to_addr, template_key, payload, attempts;

-- name: MarkNotificationSent :exec
UPDATE notification_logs SET status = 'sent', provider_ref = $2, sent_at = now(), error = ''
WHERE id = $1;

-- name: RetryNotification :exec
UPDATE notification_logs SET status = 'queued', error = $2, next_attempt_at = $3
WHERE id = $1;

-- name: FailNotification :exec
UPDATE notification_logs SET status = 'failed', error = $2
WHERE id = $1;

-- name: GetActiveGateway :one
SELECT * FROM wa_gateways WHERE tenant_id = $1 AND is_active = true ORDER BY id LIMIT 1;

-- name: GetTemplate :one
SELECT * FROM message_templates
WHERE tenant_id = $1 AND key = $2 AND channel = $3 AND is_active = true;

-- name: UpsertGateway :one
INSERT INTO wa_gateways (tenant_id, provider, config, is_active, daily_limit)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpsertTemplate :one
INSERT INTO message_templates (tenant_id, key, channel, body, is_active)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, key, channel) DO UPDATE SET body = EXCLUDED.body, is_active = EXCLUDED.is_active, updated_at = now()
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notification_logs WHERE tenant_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;
