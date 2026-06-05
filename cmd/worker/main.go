// Command worker consumes the notification queue: it polls for due jobs and
// delivers them via the tenant's WhatsApp provider with backoff retry. Other
// async jobs (OLT/ACS polling, webhook retry) will be added here.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/domain/notification"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
	"github.com/aashs25/hsbillnradius/internal/platform/postgres"
	platformredis "github.com/aashs25/hsbillnradius/internal/platform/redis"
	"github.com/aashs25/hsbillnradius/internal/ports/wa"
	"github.com/aashs25/hsbillnradius/internal/repo"
	"github.com/aashs25/hsbillnradius/internal/service/notifysvc"
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

	pool, err := postgres.New(ctx, postgres.Config{
		DSN:             cfg.Postgres.DSN,
		MaxConns:        cfg.Postgres.MaxConns,
		MinConns:        cfg.Postgres.MinConns,
		MaxConnLifetime: cfg.Postgres.MaxConnLifetime,
		MaxConnIdleTime: cfg.Postgres.MaxConnIdleTime,
		ConnectTimeout:  cfg.Postgres.ConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := platformredis.New(ctx, platformredis.Config{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize, DialTimeout: cfg.Redis.DialTimeout,
	})
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	store := repo.NewStore(pool)
	httpClient := &http.Client{Timeout: cfg.Worker.HTTPTimeout}
	clients := map[notification.Provider]wa.Client{
		notification.ProviderFonnte: wa.NewFonnte(httpClient),
		notification.ProviderWablas: wa.NewWablas(httpClient),
	}
	notifier := notifysvc.New(store.Repositories(), clients, int32(cfg.Worker.MaxAttempts), log)

	log.Info("worker started", slog.Duration("poll", cfg.Worker.PollInterval))
	ticker := time.NewTicker(cfg.Worker.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("worker shutting down")
			return nil
		case <-ticker.C:
			jobCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.PollInterval)
			n, err := notifier.ProcessDue(jobCtx, int32(cfg.Worker.Batch))
			cancel()
			if err != nil {
				log.Error("notification processing failed", slog.Any("error", err))
			} else if n > 0 {
				log.Info("processed notifications", slog.Int("count", n))
			}
		}
	}
}
