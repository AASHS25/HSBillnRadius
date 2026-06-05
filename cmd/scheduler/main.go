// Command scheduler runs periodic billing jobs: marking invoices overdue and
// scanning for expired customers to auto-isolir. Invoice generation and
// reminders are driven by the same service and can be added as further entries.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
	"github.com/aashs25/hsbillnradius/internal/platform/postgres"
	"github.com/aashs25/hsbillnradius/internal/platform/radiusclient"
	platformredis "github.com/aashs25/hsbillnradius/internal/platform/redis"
	"github.com/aashs25/hsbillnradius/internal/repo"
	"github.com/aashs25/hsbillnradius/internal/service/billingsvc"
	"github.com/aashs25/hsbillnradius/internal/service/coasvc"
)

// isolirScanLimit caps customers processed per scan tick.
const isolirScanLimit = 500

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
	repos := store.Repositories()
	coaClient := radiusclient.New(cfg.Radius.CoAPort, cfg.Radius.RequestTimeout)
	coaService := coasvc.New(repos, coaClient, cache.NewRedis(rdb), log)
	billingService := billingsvc.New(repos, store, coaService, log)

	c := cron.New()
	// Mark overdue invoices daily at 01:00.
	if _, err := c.AddFunc("0 1 * * *", func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		n, err := billingService.MarkOverdue(jobCtx)
		if err != nil {
			log.Error("mark overdue job failed", slog.Any("error", err))
			return
		}
		log.Info("mark overdue job done", slog.Int64("count", n))
	}); err != nil {
		return err
	}
	// Auto-isolir scan every 15 minutes.
	if _, err := c.AddFunc("*/15 * * * *", func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		n, err := billingService.RunIsolirScan(jobCtx, coaService, isolirScanLimit)
		if err != nil {
			log.Error("isolir scan job failed", slog.Any("error", err))
			return
		}
		if n > 0 {
			log.Info("isolir scan job done", slog.Int("isolated", n))
		}
	}); err != nil {
		return err
	}

	c.Start()
	log.Info("scheduler started", slog.Int("jobs", len(c.Entries())))

	<-ctx.Done()
	log.Info("scheduler shutting down")
	<-c.Stop().Done() // wait for running jobs to finish
	return nil
}
