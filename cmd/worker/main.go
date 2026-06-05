// Command worker consumes background jobs: WhatsApp/email notifications, webhook
// retries, OLT/ACS polling and broadcasts. Heavy or slow integrations run here
// so they never block the request path.
//
// M0 scaffolding only: it boots config + logging and blocks until shutdown. Job
// consumers (river) are wired starting in M6.
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
		slog.Error("worker-service exited with error", slog.Any("error", err))
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
		AppName: cfg.App.Name + "-worker",
		Env:     cfg.App.Env,
	})
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("worker-service starting (scaffold)")

	<-ctx.Done()
	log.Info("worker-service shutting down")
	return nil
}
