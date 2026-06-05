package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/aashs25/hsbillnradius/internal/platform/ratelimit"
)

// RateLimit returns middleware that throttles requests per client IP + path,
// responding 429 when the window limit is exceeded.
func RateLimit(limiter ratelimit.Limiter, limit int, window time.Duration, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r) + ":" + r.URL.Path
			allowed, err := limiter.Allow(r.Context(), key, limit, window)
			if err == nil && !allowed {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, http.StatusTooManyRequests, errorBody{
					Error: "too many requests, please slow down", Code: "rate_limited",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
