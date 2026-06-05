-- name: UpsertBandwidthProfile :one
INSERT INTO bandwidth_profiles (
    tenant_id, plan_id, rate_limit_rx, rate_limit_tx, burst_rx, burst_tx,
    burst_threshold_rx, burst_threshold_tx, burst_time, priority, mikrotik_rate_string
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (plan_id) DO UPDATE SET
    rate_limit_rx = EXCLUDED.rate_limit_rx,
    rate_limit_tx = EXCLUDED.rate_limit_tx,
    burst_rx = EXCLUDED.burst_rx,
    burst_tx = EXCLUDED.burst_tx,
    burst_threshold_rx = EXCLUDED.burst_threshold_rx,
    burst_threshold_tx = EXCLUDED.burst_threshold_tx,
    burst_time = EXCLUDED.burst_time,
    priority = EXCLUDED.priority,
    mikrotik_rate_string = EXCLUDED.mikrotik_rate_string,
    updated_at = now()
RETURNING *;

-- name: GetBandwidthProfileByPlan :one
SELECT * FROM bandwidth_profiles WHERE plan_id = $1;
