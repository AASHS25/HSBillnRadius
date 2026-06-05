// Command api runs the REST/WebSocket service for admin, operator, reseller and
// client-area traffic plus inbound webhooks. For M0 it exposes only health,
// readiness and metrics endpoints; feature routes are added in later milestones.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"

	"github.com/aashs25/hsbillnradius/internal/config"
	"github.com/aashs25/hsbillnradius/internal/platform/httpserver"
	"github.com/aashs25/hsbillnradius/internal/platform/logger"
	"github.com/aashs25/hsbillnradius/internal/platform/password"
	"github.com/aashs25/hsbillnradius/internal/platform/postgres"
	platformredis "github.com/aashs25/hsbillnradius/internal/platform/redis"
	"github.com/aashs25/hsbillnradius/internal/platform/token"
	"github.com/aashs25/hsbillnradius/internal/repo"
	"github.com/aashs25/hsbillnradius/internal/service/authsvc"
	"github.com/aashs25/hsbillnradius/internal/transport/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api-service exited with error", slog.Any("error", err))
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
		AppName: cfg.App.Name,
		Env:     cfg.App.Env,
	})
	slog.SetDefault(log)

	// Cancel the root context on SIGINT/SIGTERM for graceful shutdown.
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

	// Compose the auth stack: repositories, crypto, token manager, services.
	store := repo.NewStore(pool)
	hasher := password.NewHasher()
	tokens := token.NewManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.AccessTokenTTL)
	authService := authsvc.New(store.Repositories(), store, hasher, tokens, cfg.Auth.RefreshTokenTTL, log)
	api := httpapi.New(authService, tokens, log)

	router := newRouter(cfg, log, pool, rdb, api)

	srv := httpserver.New(httpserver.Config{
		Addr:            cfg.HTTP.Addr(),
		ReadTimeout:     cfg.HTTP.ReadTimeout,
		WriteTimeout:    cfg.HTTP.WriteTimeout,
		IdleTimeout:     cfg.HTTP.IdleTimeout,
		ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
	}, router, log)

	return srv.Run(ctx)
}

// newRouter assembles the chi router, base middleware stack and API routes.
func newRouter(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool, rdb *goredis.Client, api *httpapi.API) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Real client IP is resolved at the edge (LB) and will be parsed from a
	// trusted X-Forwarded-For chain in a later milestone; chi's RealIP is
	// deprecated because it trusts spoofable headers unconditionally.
	r.Use(httpserver.RequestLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(cfg.HTTP.RequestTimeout))

	hc := httpserver.NewHealthChecker(pool, rdb, log)
	r.Get("/healthz", hc.Liveness)
	r.Get("/readyz", hc.Readiness)
	r.Handle("/metrics", promhttp.Handler())

	api.Mount(r)

	return r
}
