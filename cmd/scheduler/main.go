// Command scheduler runs periodic jobs: invoice generation, payment reminders,
// due-date/isolir scans, voucher cleanup and backup triggers.
//
// M0 scaffolding only: it boots config + logging and blocks until shutdown. Cron
// entries are wired starting in M5.
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
		slog.Error("scheduler-service exited with error", slog.Any("error", err))
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
		AppName: cfg.App.Name + "-scheduler",
		Env:     cfg.App.Env,
	})
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("scheduler-service starting (scaffold)")

	<-ctx.Done()
	log.Info("scheduler-service shutting down")
	return nil
}
