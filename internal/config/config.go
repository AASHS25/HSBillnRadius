// Package config loads and validates all runtime configuration from the
// environment (12-factor). It owns every env-tagged struct so the rest of the
// codebase depends on plain data, not on the env parsing library.
package config

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the root configuration aggregate shared by every binary. Each
// binary reads only the sections it needs.
type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Radius   RadiusConfig
	Auth     AuthConfig
}

// AuthConfig configures JWT issuance and refresh-token lifetimes.
type AuthConfig struct {
	JWTSecret       string        `env:"JWT_SECRET" envDefault:"dev-insecure-secret-change-me-please!"`
	JWTIssuer       string        `env:"JWT_ISSUER" envDefault:"billing-radius"`
	AccessTokenTTL  time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
}

// defaultJWTSecret must never be used in production; Validate enforces this.
const defaultJWTSecret = "dev-insecure-secret-change-me-please!"

// AppConfig holds process-wide identity and logging settings.
type AppConfig struct {
	Name      string `env:"APP_NAME" envDefault:"billing-radius"`
	Env       string `env:"APP_ENV" envDefault:"development"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
}

// HTTPConfig configures the api-service HTTP listener and its timeouts.
type HTTPConfig struct {
	Host            string        `env:"HTTP_HOST" envDefault:"0.0.0.0"`
	Port            int           `env:"HTTP_PORT" envDefault:"8080"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"20s"`
	RequestTimeout  time.Duration `env:"HTTP_REQUEST_TIMEOUT" envDefault:"30s"`
}

// Addr returns the host:port the HTTP server binds to.
func (c HTTPConfig) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// PostgresConfig configures the pgx connection pool. A full DSN is preferred
// over individual fields to avoid URL-encoding pitfalls with secrets.
type PostgresConfig struct {
	DSN             string        `env:"POSTGRES_DSN" envDefault:"postgres://postgres:postgres@localhost:5432/billing_radius?sslmode=disable"`
	MaxConns        int32         `env:"POSTGRES_MAX_CONNS" envDefault:"10"`
	MinConns        int32         `env:"POSTGRES_MIN_CONNS" envDefault:"2"`
	MaxConnLifetime time.Duration `env:"POSTGRES_MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnIdleTime time.Duration `env:"POSTGRES_MAX_CONN_IDLE_TIME" envDefault:"30m"`
	ConnectTimeout  time.Duration `env:"POSTGRES_CONNECT_TIMEOUT" envDefault:"5s"`
}

// RedisConfig configures the shared Redis client (cache, locks, pub/sub).
type RedisConfig struct {
	Addr        string        `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	Password    string        `env:"REDIS_PASSWORD"`
	DB          int           `env:"REDIS_DB" envDefault:"0"`
	PoolSize    int           `env:"REDIS_POOL_SIZE" envDefault:"10"`
	DialTimeout time.Duration `env:"REDIS_DIAL_TIMEOUT" envDefault:"5s"`
}

// RadiusConfig configures the radius-service UDP listeners, worker pool and
// lookup cache TTLs.
type RadiusConfig struct {
	AuthAddr       string        `env:"RADIUS_AUTH_ADDR" envDefault:"0.0.0.0:1812"`
	AcctAddr       string        `env:"RADIUS_ACCT_ADDR" envDefault:"0.0.0.0:1813"`
	CoAPort        int           `env:"RADIUS_COA_PORT" envDefault:"3799"`
	Workers        int           `env:"RADIUS_WORKERS" envDefault:"64"`
	RequestTimeout time.Duration `env:"RADIUS_REQUEST_TIMEOUT" envDefault:"3s"`
	NasCacheTTL    time.Duration `env:"RADIUS_NAS_CACHE_TTL" envDefault:"60s"`
	UserCacheTTL   time.Duration `env:"RADIUS_USER_CACHE_TTL" envDefault:"45s"`
}

// Load reads configuration from the environment, applying a best-effort .env
// file in development, then validates it. It fails fast so misconfiguration is
// caught at boot rather than at first request.
func Load() (*Config, error) {
	// Best-effort: a missing .env is normal in production. godotenv does not
	// override variables already present in the environment.
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config from env: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

// validEnvs and validLogLevels constrain free-form string settings.
var (
	validEnvs      = []string{"development", "staging", "production"}
	validLogLevels = []string{"debug", "info", "warn", "error"}
	validLogFmts   = []string{"json", "text"}
)

// Validate enforces invariants that the type system cannot, returning a single
// error describing the first problem found.
func (c Config) Validate() error {
	if !slices.Contains(validEnvs, c.App.Env) {
		return fmt.Errorf("APP_ENV %q must be one of %v", c.App.Env, validEnvs)
	}
	if !slices.Contains(validLogLevels, c.App.LogLevel) {
		return fmt.Errorf("LOG_LEVEL %q must be one of %v", c.App.LogLevel, validLogLevels)
	}
	if !slices.Contains(validLogFmts, c.App.LogFormat) {
		return fmt.Errorf("LOG_FORMAT %q must be one of %v", c.App.LogFormat, validLogFmts)
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP_PORT %d out of range", c.HTTP.Port)
	}
	if c.Postgres.DSN == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	if c.Postgres.MaxConns < c.Postgres.MinConns {
		return fmt.Errorf("POSTGRES_MAX_CONNS (%d) must be >= POSTGRES_MIN_CONNS (%d)",
			c.Postgres.MaxConns, c.Postgres.MinConns)
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	if c.Auth.AccessTokenTTL <= 0 {
		return fmt.Errorf("ACCESS_TOKEN_TTL must be positive")
	}
	if c.Auth.RefreshTokenTTL <= c.Auth.AccessTokenTTL {
		return fmt.Errorf("REFRESH_TOKEN_TTL must be greater than ACCESS_TOKEN_TTL")
	}
	if c.IsProduction() {
		if c.Auth.JWTSecret == defaultJWTSecret {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if len(c.Auth.JWTSecret) < 32 {
			return fmt.Errorf("JWT_SECRET must be at least 32 bytes in production")
		}
	}
	return nil
}

// IsProduction reports whether the process runs in the production environment.
func (c Config) IsProduction() bool { return c.App.Env == "production" }
