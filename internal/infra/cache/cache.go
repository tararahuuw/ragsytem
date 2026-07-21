// Package cache is the caching adapter (Redis). It follows the mockable pattern
// of the other infra adapters (ai/email/sentry/minio): an empty REDIS_ADDR (or
// CACHE_ENABLED=false) yields a no-op cache where every Get is a miss, so the app
// runs unchanged without Redis.
//
// Cache is an OPTIMIZATION ONLY — Postgres stays the source of truth. Every
// operation is designed to FAIL OPEN: callers log cache errors and fall back to
// the database, so a Redis outage degrades performance, never correctness.
package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// Cache is a minimal key/value store with TTL.
type Cache interface {
	// Get returns (value, true, nil) on a hit and (nil, false, nil) on a miss.
	// A non-nil error means the backend failed — callers should fall back to DB.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Close() error
}

// New builds a Cache from config: a real Redis client when caching is active,
// otherwise a no-op (disabled) cache.
func New(cfg *config.Config) Cache {
	if !cfg.CacheActive() {
		slog.Warn("cache disabled: REDIS_ADDR kosong / CACHE_ENABLED=false — memakai no-op (semua baca langsung ke DB)")
		return noop{}
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	slog.Info("cache enabled (redis)", "addr", cfg.RedisAddr, "db", cfg.RedisDB, "ttl", cfg.CacheTTL)
	return &redisCache{client: client}
}

type redisCache struct{ client *redis.Client }

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil // miss
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *redisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, val, ttl).Err()
}

func (c *redisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *redisCache) Close() error { return c.client.Close() }

// noop is the disabled cache: every Get is a miss; writes are dropped.
type noop struct{}

func (noop) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (noop) Delete(context.Context, ...string) error                  { return nil }
func (noop) Close() error                                             { return nil }
