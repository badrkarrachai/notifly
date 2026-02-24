package notification

import (
	"context"
	"time"
)

// NotificationStore defines the contract for persisting notification records.
// Implementations live in infra/store/ (e.g., Supabase).
type NotificationStore interface {
	// Create inserts a new notification log record.
	Create(ctx context.Context, log *NotificationLog) error

	// GetByID retrieves a notification log by its ID.
	GetByID(ctx context.Context, id string) (*NotificationLog, error)

	// GetByIdempotencyKey retrieves a notification log by its idempotency key.
	// Returns nil, nil if no record is found.
	GetByIdempotencyKey(ctx context.Context, key string) (*NotificationLog, error)

	// UpdateStatus updates the status of a notification log.
	UpdateStatus(ctx context.Context, id string, status NotificationStatus, providerID string, errMsg string) error

	// UpdateWebhookStatus updates the status of a notification based on provider ID (for webhook events).
	UpdateWebhookStatus(ctx context.Context, providerID string, status NotificationStatus) error

	// List retrieves notification logs with pagination and filtering.
	List(ctx context.Context, filter ListFilter) ([]*NotificationLog, int, error)

	// ListStale retrieves notification logs stuck in queued/processing for longer
	// than the given threshold. Used by the reaper for reconciliation.
	ListStale(ctx context.Context, olderThan time.Time, limit int) ([]*NotificationLog, error)
}
