// Package cache provides a tiny byte-blob cache abstraction with a Redis
// implementation and an in-memory no-op, used to cache hot RADIUS lookups.
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is a minimal key/value cache of byte blobs with TTL.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration)
	Del(ctx context.Context, key string)
}

// Redis implements Cache over a go-redis client. Cache errors are swallowed
// (degraded mode): a cache outage must never break authentication.
type Redis struct{ client *redis.Client }

// NewRedis wraps a redis client as a Cache.
func NewRedis(client *redis.Client) *Redis { return &Redis{client: client} }

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

func (r *Redis) Set(ctx context.Context, key string, val []byte, ttl time.Duration) {
	_ = r.client.Set(ctx, key, val, ttl).Err()
}

func (r *Redis) Del(ctx context.Context, key string) {
	_ = r.client.Del(ctx, key).Err()
}

// Noop is a cache that never stores anything (every Get misses).
type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool)         { return nil, false }
func (Noop) Set(context.Context, string, []byte, time.Duration) {}
func (Noop) Del(context.Context, string)                        {}
