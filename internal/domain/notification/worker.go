package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"notifly/internal/common"
)

// Worker processes notification tasks from the queue.
// It picks up a task, fetches the log from the store, renders the template,
// sends via the appropriate provider, and updates the log status.
type Worker struct {
	store     NotificationStore
	renderer  TemplateRenderer
	providers map[Channel]Provider
}

// NewWorker creates a new notification worker.
func NewWorker(store NotificationStore, renderer TemplateRenderer, providers ...Provider) *Worker {
	pm := make(map[Channel]Provider, len(providers))
	for _, p := range providers {
		pm[p.Channel()] = p
	}
	return &Worker{
		store:     store,
		renderer:  renderer,
		providers: pm,
	}
}

// ProcessTask handles a send notification task from the queue.
func (w *Worker) ProcessTask(ctx context.Context, logID string) error {
	start := time.Now()

	// Fetch the notification log
	notifLog, err := w.store.GetByID(ctx, logID)
	if err != nil {
		return fmt.Errorf("fetching notification log %s: %w", logID, err)
	}

	if notifLog == nil {
		slog.Error("notification log not found", "log_id", logID)
		return fmt.Errorf("notification log not found: %s", logID)
	}

	// Update status to processing
	if err := w.store.UpdateStatus(ctx, logID, StatusProcessing, "", ""); err != nil {
		slog.Error("failed to update status to processing", "log_id", logID, "error", err)
	}

	channel := Channel(notifLog.Channel)
	notifType := NotificationType(notifLog.Type)

	// Validate notification type
	if !IsValidType(notifType) {
		errMsg := fmt.Sprintf("unsupported notification type: %s", notifType)
		_ = w.store.UpdateStatus(ctx, logID, StatusFailed, "", errMsg)
		return common.NewValidationError(errMsg)
	}

	// Resolve the channel provider
	provider, ok := w.providers[channel]
	if !ok {
		errMsg := fmt.Sprintf("unsupported channel: %s", channel)
		_ = w.store.UpdateStatus(ctx, logID, StatusFailed, "", errMsg)
		return common.NewValidationError(errMsg)
	}

	// Render the template
	subject, html, text, err := w.renderer.Render(notifType, notifLog.TemplateData)
	if err != nil {
		errMsg := fmt.Sprintf("rendering template: %s", err.Error())
		_ = w.store.UpdateStatus(ctx, logID, StatusFailed, "", errMsg)
		return fmt.Errorf("rendering template %s: %w", notifType, err)
	}

	// Build the message
	msg := &Message{
		To:      notifLog.Recipient,
		Subject: subject,
		HTML:    html,
		Text:    text,
	}

	// Send via the channel provider
	providerID, err := provider.Send(ctx, msg)
	if err != nil {
		errMsg := fmt.Sprintf("provider error: %s", err.Error())
		_ = w.store.UpdateStatus(ctx, logID, StatusFailed, "", errMsg)

		slog.Error("notification delivery failed",
			"log_id", logID,
			"channel", channel,
			"type", notifType,
			"to", notifLog.Recipient,
			"error", err,
			"duration", time.Since(start),
		)
		return common.NewProviderError(string(channel), err.Error())
	}

	// Update log with success
	if err := w.store.UpdateStatus(ctx, logID, StatusSent, providerID, ""); err != nil {
		slog.Error("failed to update status to sent", "log_id", logID, "error", err)
	}

	slog.Info("notification sent",
		"log_id", logID,
		"channel", channel,
		"type", notifType,
		"to", notifLog.Recipient,
		"provider_id", providerID,
		"duration", time.Since(start),
	)

	return nil
}
