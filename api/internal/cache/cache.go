package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

// Init connects to Redis. If redisURL is empty or Redis is unreachable,
// caching is silently disabled and the app works normally without it.
func Init(redisURL string) {
	if redisURL == "" {
		return
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("cache: invalid Redis URL, caching disabled", "error", err)
		return
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		slog.Warn("cache: Redis unreachable, caching disabled", "error", err)
		return
	}
	Client = c
	slog.Info("cache: Redis connected")
}

// Get returns the cached string and true on hit, or "", false on miss/error.
func Get(ctx context.Context, key string) (string, bool) {
	if Client == nil {
		return "", false
	}
	val, err := Client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// Set stores value with the given TTL. Errors are logged but not propagated.
func Set(ctx context.Context, key, value string, ttl time.Duration) {
	if Client == nil {
		return
	}
	if err := Client.Set(ctx, key, value, ttl).Err(); err != nil {
		slog.Warn("cache: set failed", "key", key, "error", err)
	}
}

// DeletePattern removes all keys matching a glob pattern (e.g. "videos:admin:*").
func DeletePattern(ctx context.Context, pattern string) {
	if Client == nil {
		return
	}
	var keys []string
	iter := Client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		if err := Client.Del(ctx, keys...).Err(); err != nil {
			slog.Warn("cache: delete pattern failed", "pattern", pattern, "error", err)
		}
	}
}
