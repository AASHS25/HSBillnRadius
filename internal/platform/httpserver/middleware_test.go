package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestLogger_PassesThroughAndLogs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	handler.ServeHTTP(rec, req)

	// The inner handler's response is preserved.
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "hi", rec.Body.String())

	// A single structured line describing the request was emitted.
	logged := buf.String()
	assert.Contains(t, logged, "http request")
	assert.Contains(t, logged, "/ping")
	assert.Contains(t, logged, "GET")
	assert.Contains(t, logged, "418")
}
