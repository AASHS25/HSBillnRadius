package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aashs25/hsbillnradius/internal/transport/httpapi"
)

type fakeLimiter struct {
	count int
	limit int
}

func (f *fakeLimiter) Allow(context.Context, string, int, time.Duration) (bool, error) {
	f.count++
	return f.count <= f.limit, nil
}

func TestRateLimit_Throttles(t *testing.T) {
	lim := &fakeLimiter{limit: 1}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := httpapi.RateLimit(lim, 1, time.Minute, log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request is throttled.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
}
