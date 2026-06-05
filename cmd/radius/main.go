// Command radius runs the RADIUS Auth + Accounting server and the CoA/Disconnect
// sender. It is deliberately a separate process from the api-service so that an
// api crash never interrupts customer authentication.
//
// M0 scaffolding only: it boots config + logging and blocks until a shutdown
// signal. The UDP listeners and handlers are implemented in M3/M4.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("radius-service exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(logger.Options{
		Level:   cfg.App.LogLevel,
		Format:  cfg.App.LogFormat,
		AppName: cfg.App.Name + "-radius",
		Env:     cfg.App.Env,
	})
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("radius-service starting (scaffold)",
		slog.String("auth_addr", cfg.Radius.AuthAddr),
		slog.String("acct_addr", cfg.Radius.AcctAddr),
		slog.Int("coa_port", cfg.Radius.CoAPort),
	)

	<-ctx.Done()
	log.Info("radius-service shutting down")
	return nil
}
