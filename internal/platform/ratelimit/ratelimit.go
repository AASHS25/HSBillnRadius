// Package ratelimit provides a Redis fixed-window rate limiter used to throttle
// sensitive endpoints (login, register) per client.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter decides whether a request identified by key is allowed.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RedisLimiter is a fixed-window limiter backed by Redis INCR/EXPIRE.
type RedisLimiter struct{ client *redis.Client }

// NewRedis builds a RedisLimiter.
func NewRedis(client *redis.Client) *RedisLimiter { return &RedisLimiter{client: client} }

// Allow increments the window counter and reports whether it is within limit.
// On a Redis error it fails open (allows) so a cache outage never locks users out.
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := "rl:" + key
	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return true, nil // fail open
	}
	if count == 1 {
		_ = l.client.Expire(ctx, redisKey, window).Err()
	}
	return count <= int64(limit), nil
}

// AllowAll is a no-op limiter (used in tests/dev).
type AllowAll struct{}

// Allow always permits the request.
func (AllowAll) Allow(context.Context, string, int, time.Duration) (bool, error) {
	return true, nil
}
