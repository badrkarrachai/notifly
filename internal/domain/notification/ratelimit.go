package notification

import "context"

// RecipientRateLimiter defines the contract for per-recipient rate limiting.
// Implementations live in infra/ratelimit/.
type RecipientRateLimiter interface {
	// Allow checks whether a notification can be sent to the given recipient.
	// Returns true if the notification is allowed, false if rate limited.
	Allow(ctx context.Context, recipient string) (bool, error)
}
