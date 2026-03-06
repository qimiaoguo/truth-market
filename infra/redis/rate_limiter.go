package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const rateLimitKeyPrefix = "ratelimit:"

// RateLimiter implements a sliding-window rate limiter using Redis INCR and
// EXPIRE. Each key tracks the number of requests within a fixed time window.
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter returns a new RateLimiter backed by the given Redis client.
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow checks whether a request identified by key is within the rate limit.
// It increments the counter for the current window and returns true if the
// count is at or below the limit. The window resets automatically via EXPIRE.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := rateLimitKeyPrefix + key

	count, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		if err := r.client.Expire(ctx, redisKey, window).Err(); err != nil {
			return false, err
		}
	}

	return count <= int64(limit), nil
}
