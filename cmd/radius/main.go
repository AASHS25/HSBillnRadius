// Command radius runs the RADIUS Auth server and (from M4) Accounting + CoA. It
// is a separate process from the api-service so an api crash never interrupts
// customer authentication.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/platform/cache"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
	"github.com/aashs25/hsbillnradius/internal/platform/postgres"
	"github.com/aashs25/hsbillnradius/internal/platform/radiusserver"
	platformredis "github.com/aashs25/hsbillnradius/internal/platform/redis"
	"github.com/aashs25/hsbillnradius/internal/repo"
	"github.com/aashs25/hsbillnradius/internal/service/radiussvc"
	"github.com/aashs25/hsbillnradius/internal/transport/radiusapi"
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
	log.Info("connected to postgres")

	rdb, err := platformredis.New(ctx, platformredis.Config{
		Addr:        cfg.Redis.Addr,
		Password:    cfg.Redis.Password,
		DB:          cfg.Redis.DB,
		PoolSize:    cfg.Redis.PoolSize,
		DialTimeout: cfg.Redis.DialTimeout,
	})
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	log.Info("connected to redis")

	store := repo.NewStore(pool)
	svc := radiussvc.New(store.Repositories(), cache.NewRedis(rdb), cfg.Radius.NasCacheTTL, cfg.Radius.UserCacheTTL, log)
	handler := radiusapi.NewAuthHandler(svc, log)

	server := radiusserver.New(cfg.Radius.AuthAddr, cfg.Radius.Workers, cfg.Radius.RequestTimeout, handler.Handle, log)
	return server.Run(ctx)
}
