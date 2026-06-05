package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// healthCheckTimeout bounds each dependency probe in the readiness check.
const healthCheckTimeout = 2 * time.Second

// HealthChecker serves liveness and readiness probes. Liveness reports only
// that the process is up; readiness verifies that PostgreSQL and Redis are
// reachable. Either dependency may be nil (e.g. in tests or in binaries that
// do not use it), in which case it is skipped.
type HealthChecker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  *slog.Logger
}

// NewHealthChecker wires the dependencies probed by the readiness endpoint.
func NewHealthChecker(pool *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *HealthChecker {
	return &HealthChecker{pool: pool, rdb: rdb, log: log}
}

// Liveness always returns 200 while the process is able to serve requests.
func (h *HealthChecker) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness probes dependencies and returns 200 only when all are healthy,
// otherwise 503 with a per-dependency status map.
func (h *HealthChecker) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if h.pool != nil {
		if err := h.pool.Ping(ctx); err != nil {
			checks["postgres"] = "down"
			healthy = false
			h.log.WarnContext(ctx, "readiness: postgres ping failed", slog.Any("error", err))
		} else {
			checks["postgres"] = "up"
		}
	}

	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "down"
			healthy = false
			h.log.WarnContext(ctx, "readiness: redis ping failed", slog.Any("error", err))
		} else {
			checks["redis"] = "up"
		}
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"status": readyWord(healthy), "checks": checks})
}

func readyWord(ok bool) string {
	if ok {
		return "ready"
	}
	return "unavailable"
}

// writeJSON serializes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
