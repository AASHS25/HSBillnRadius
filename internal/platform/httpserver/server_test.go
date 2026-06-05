package httpserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RunShutsDownOnContextCancel(t *testing.T) {
	srv := New(Config{
		Addr:            "127.0.0.1:0", // random free port
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		IdleTimeout:     time.Second,
		ShutdownTimeout: 2 * time.Second,
	}, http.NewServeMux(), discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	// Let the listener come up, then trigger graceful shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err, "graceful shutdown should not error")
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down within the deadline")
	}
}

func TestServer_RunReturnsListenError(t *testing.T) {
	// An invalid address makes ListenAndServe fail immediately; Run must
	// surface that error rather than block.
	srv := New(Config{
		Addr:            "127.0.0.1:not-a-port",
		ShutdownTimeout: time.Second,
	}, http.NewServeMux(), discardLogger())

	err := srv.Run(context.Background())
	assert.Error(t, err)
}
