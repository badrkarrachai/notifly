package notification

import (
	"context"
	"fmt"
	"log/slog"

	"notifly/internal/common"
)

// Enqueuer defines the contract for enqueuing notification tasks.
// This allows the service to be decoupled from the specific queue implementation.
type Enqueuer interface {
	EnqueueSendNotification(logID string) error
}

// Service orchestrates notification business logic.
// In the async flow: validate → check idempotency → check rate limit → create log → enqueue.
type Service struct {
	store       NotificationStore
	enqueuer    Enqueuer
	rateLimiter RecipientRateLimiter
}

// NewService creates a new notification service.
func NewService(store NotificationStore, enqueuer Enqueuer, rateLimiter RecipientRateLimiter) *Service {
	return &Service{
		store:       store,
		enqueuer:    enqueuer,
		rateLimiter: rateLimiter,
	}
}

// Enqueue validates a notification request, checks idempotency and rate limits,
// creates a log record, and enqueues the task for async processing.
func (s *Service) Enqueue(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// Validate notification type
	if !IsValidType(req.Type) {
		return nil, common.NewValidationError(fmt.Sprintf("unsupported notification type: %s", req.Type))
	}

	// Check idempotency — if a request with the same key already exists, return the existing result
	if req.IdempotencyKey != "" {
		existing, err := s.store.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			slog.Error("idempotency check failed", "key", req.IdempotencyKey, "error", err)
			// Don't fail the request — proceed without idempotency protection
		}
		if existing != nil {
			slog.Info("idempotent request — returning existing result",
				"idempotency_key", req.IdempotencyKey,
				"existing_id", existing.ID,
				"existing_status", existing.Status,
			)
			return &SendResponse{
				ID:             existing.ID,
				IdempotencyKey: existing.IdempotencyKey,
				Channel:        existing.Channel,
				Status:         string(existing.Status),
			}, nil
		}
	}

	// Check per-recipient rate limit
	if s.rateLimiter != nil {
		allowed, err := s.rateLimiter.Allow(ctx, req.To)
		if err != nil {
			slog.Error("rate limit check failed, proceeding without limit", "recipient", req.To, "error", err)
			// Fail open — don't block the request when Redis is down
		} else if !allowed {
			return nil, common.NewValidationError(fmt.Sprintf("rate limit exceeded for recipient: %s", req.To))
		}
	}

	// Create the notification log
	notifLog := &NotificationLog{
		IdempotencyKey: req.IdempotencyKey,
		Channel:        string(req.Channel),
		Type:           string(req.Type),
		Recipient:      req.To,
		TemplateData:   req.Data,
		Status:         StatusQueued,
	}

	if err := s.store.Create(ctx, notifLog); err != nil {
		return nil, fmt.Errorf("creating notification log: %w", err)
	}

	// Enqueue the task for async processing
	if err := s.enqueuer.EnqueueSendNotification(notifLog.ID); err != nil {
		// Update log status to failed since we couldn't enqueue
		_ = s.store.UpdateStatus(ctx, notifLog.ID, StatusFailed, "", "failed to enqueue: "+err.Error())
		return nil, fmt.Errorf("enqueuing notification: %w", err)
	}

	slog.Info("notification enqueued",
		"id", notifLog.ID,
		"channel", req.Channel,
		"type", req.Type,
		"to", req.To,
	)

	return &SendResponse{
		ID:             notifLog.ID,
		IdempotencyKey: notifLog.IdempotencyKey,
		Channel:        string(req.Channel),
		Status:         string(StatusQueued),
	}, nil
}

// GetNotification retrieves a notification log by ID.
func (s *Service) GetNotification(ctx context.Context, id string) (*NotificationLog, error) {
	notifLog, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching notification: %w", err)
	}
	if notifLog == nil {
		return nil, common.NewNotFoundError("notification", id)
	}
	return notifLog, nil
}

// ListNotifications retrieves notification logs with pagination and filtering.
func (s *Service) ListNotifications(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	logs, total, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing notifications: %w", err)
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	return &ListResponse{
		Notifications: logs,
		Total:         total,
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	}, nil
}

// HandleWebhookEvent processes a delivery status update from a provider webhook.
func (s *Service) HandleWebhookEvent(ctx context.Context, providerID string, status NotificationStatus) error {
	if providerID == "" {
		return common.NewValidationError("provider_id is required")
	}

	if err := s.store.UpdateWebhookStatus(ctx, providerID, status); err != nil {
		return fmt.Errorf("updating webhook status: %w", err)
	}

	slog.Info("webhook status updated",
		"provider_id", providerID,
		"status", status,
	)

	return nil
}
