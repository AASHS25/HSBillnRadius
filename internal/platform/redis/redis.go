// Package redis provides a configured go-redis client used for cache,
// rate-limiting, locks and pub/sub across the platform.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds the Redis client settings, mapped from config.RedisConfig.
type Config struct {
	Addr        string
	Password    string
	DB          int
	PoolSize    int
	DialTimeout time.Duration
}

// New constructs a Redis client and verifies connectivity with a deadline
// derived from DialTimeout.
func New(ctx context.Context, cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        cfg.Addr,
		Password:    cfg.Password,
		DB:          cfg.DB,
		PoolSize:    cfg.PoolSize,
		DialTimeout: cfg.DialTimeout,
	})

	pingCtx := ctx
	if cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.DialTimeout)
		defer cancel()
	}
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
