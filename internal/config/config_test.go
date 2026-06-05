package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// With no overrides the defaults must produce a valid config.
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "billing-radius", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "0.0.0.0:8080", cfg.HTTP.Addr())
	assert.Equal(t, 30*time.Second, cfg.HTTP.RequestTimeout)
	assert.Equal(t, int32(10), cfg.Postgres.MaxConns)
	assert.False(t, cfg.IsProduction())
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("HTTP_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_MAX_CONNS", "50")
	t.Setenv("REDIS_ADDR", "redis:6380")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.IsProduction())
	assert.Equal(t, "127.0.0.1:9090", cfg.HTTP.Addr())
	assert.Equal(t, int32(50), cfg.Postgres.MaxConns)
	assert.Equal(t, "redis:6380", cfg.Redis.Addr)
}

func TestValidate_Errors(t *testing.T) {
	tests := map[string]func(*Config){
		"bad env":          func(c *Config) { c.App.Env = "prod" },
		"bad log level":    func(c *Config) { c.App.LogLevel = "verbose" },
		"bad log format":   func(c *Config) { c.App.LogFormat = "xml" },
		"port too high":    func(c *Config) { c.HTTP.Port = 70000 },
		"port too low":     func(c *Config) { c.HTTP.Port = 0 },
		"empty dsn":        func(c *Config) { c.Postgres.DSN = "" },
		"conns inverted":   func(c *Config) { c.Postgres.MinConns = 20; c.Postgres.MaxConns = 10 },
		"empty redis addr": func(c *Config) { c.Redis.Addr = "" },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			mutate(&cfg)
			assert.Error(t, cfg.Validate())
		})
	}
}

func TestValidate_OK(t *testing.T) {
	cfg := validConfig()
	assert.NoError(t, cfg.Validate())
}

// validConfig returns a minimal config that passes validation, used as the
// baseline for negative validation tests.
func validConfig() Config {
	return Config{
		App:  AppConfig{Env: "development", LogLevel: "info", LogFormat: "json"},
		HTTP: HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Postgres: PostgresConfig{
			DSN:      "postgres://localhost:5432/db",
			MaxConns: 10,
			MinConns: 2,
		},
		Redis: RedisConfig{Addr: "localhost:6379"},
	}
}
