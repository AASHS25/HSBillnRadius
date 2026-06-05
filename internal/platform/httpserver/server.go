// Package httpserver wraps net/http with sane timeouts, structured request
// logging, health endpoints and graceful shutdown. The api-service composes a
// chi router and hands it to Run.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Config configures the HTTP listener and its timeouts.
type Config struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Server owns the underlying *http.Server and orchestrates its lifecycle.
type Server struct {
	httpServer      *http.Server
	log             *slog.Logger
	shutdownTimeout time.Duration
}

// New builds a Server bound to cfg.Addr serving handler.
func New(cfg Config, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		log:             log,
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// Run starts the server and blocks until ctx is cancelled or the listener
// fails. On cancellation it drains in-flight requests within ShutdownTimeout.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", slog.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		s.log.Info("http server shutting down")
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		return nil
	}
}
