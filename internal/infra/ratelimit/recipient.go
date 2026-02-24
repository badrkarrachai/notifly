package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"notifly/internal/domain/notification"

	"github.com/redis/go-redis/v9"
)

var _ notification.RecipientRateLimiter = (*RedisRecipientLimiter)(nil)

// RedisRecipientLimiter enforces per-recipient notification rate limits using Redis sorted sets.
// It uses a sliding window approach: each notification is a member scored by its timestamp.
type RedisRecipientLimiter struct {
	client     *redis.Client
	maxPerHour int
	window     time.Duration
}

// NewRedisRecipientLimiter creates a new Redis-based per-recipient rate limiter.
func NewRedisRecipientLimiter(redisAddr, password string, db int, maxPerHour int) *RedisRecipientLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})

	return &RedisRecipientLimiter{
		client:     client,
		maxPerHour: maxPerHour,
		window:     time.Hour,
	}
}

// Allow checks whether a notification can be sent to the given recipient.
// Uses a Redis sorted set with timestamps as scores for a sliding window counter.
func (r *RedisRecipientLimiter) Allow(ctx context.Context, recipient string) (bool, error) {
	key := fmt.Sprintf("notifly:ratelimit:%s", recipient)
	now := time.Now()
	windowStart := now.Add(-r.window)

	pipe := r.client.Pipeline()

	// Remove expired entries (outside the sliding window)
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count remaining entries in the window
	countCmd := pipe.ZCard(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("checking recipient rate limit: %w", err)
	}

	count := countCmd.Val()

	// If at or over the limit, deny
	if count >= int64(r.maxPerHour) {
		return false, nil
	}

	// Generate a unique member to avoid collisions on concurrent requests
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	member := redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d:%s", now.UnixNano(), hex.EncodeToString(randBytes)),
	}
	pipe2 := r.client.Pipeline()
	pipe2.ZAdd(ctx, key, member)
	pipe2.Expire(ctx, key, r.window+time.Minute) // TTL slightly longer than window for cleanup

	_, err = pipe2.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("recording rate limit entry: %w", err)
	}

	return true, nil
}

// Close closes the Redis connection.
func (r *RedisRecipientLimiter) Close() error {
	return r.client.Close()
}
