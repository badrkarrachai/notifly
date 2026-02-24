package notification

import (
	"context"
	"log/slog"
	"time"
)

// ReaperConfig holds configuration for the stale task reaper.
type ReaperConfig struct {
	// Interval is how often the reaper scans for stale tasks.
	Interval time.Duration

	// StaleThreshold is how long a task can stay in queued/processing
	// before the reaper considers it stale and re-enqueues it.
	StaleThreshold time.Duration

	// BatchSize is the maximum number of stale tasks to recover per cycle.
	BatchSize int
}

// Reaper periodically scans the notification store for stuck tasks
// and re-enqueues them. This ensures no notification is ever permanently
// lost, even if Redis data is wiped or a worker crashes without recovery.
//
// This implements the "database reconciliation" pattern:
// the database (Supabase) is the source of truth, and the reaper
// reconciles it with the queue (Redis) on a timer.
type Reaper struct {
	store    NotificationStore
	enqueuer Enqueuer
	config   ReaperConfig
}

// NewReaper creates a new stale task reaper.
func NewReaper(store NotificationStore, enqueuer Enqueuer, cfg ReaperConfig) *Reaper {
	// Sensible defaults
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.StaleThreshold <= 0 {
		cfg.StaleThreshold = 10 * time.Minute
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}

	return &Reaper{
		store:    store,
		enqueuer: enqueuer,
		config:   cfg,
	}
}

// Run starts the reaper loop. It blocks until the context is cancelled.
// Should be called in a goroutine.
func (r *Reaper) Run(ctx context.Context) {
	slog.Info("reaper started",
		"interval", r.config.Interval,
		"stale_threshold", r.config.StaleThreshold,
		"batch_size", r.config.BatchSize,
	)

	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("reaper stopped")
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

// sweep performs one reaper cycle: find stale tasks and re-enqueue them.
func (r *Reaper) sweep(ctx context.Context) {
	olderThan := time.Now().Add(-r.config.StaleThreshold)

	staleLogs, err := r.store.ListStale(ctx, olderThan, r.config.BatchSize)
	if err != nil {
		slog.Error("reaper: failed to list stale tasks", "error", err)
		return
	}

	if len(staleLogs) == 0 {
		return // Nothing to do — the common case
	}

	slog.Warn("reaper: found stale tasks", "count", len(staleLogs))

	recovered := 0
	for _, notifLog := range staleLogs {
		// Reset status to queued before re-enqueuing so the worker
		// picks it up cleanly.
		if err := r.store.UpdateStatus(ctx, notifLog.ID, StatusQueued, "", ""); err != nil {
			slog.Error("reaper: failed to reset status",
				"log_id", notifLog.ID,
				"error", err,
			)
			continue
		}

		if err := r.enqueuer.EnqueueSendNotification(notifLog.ID); err != nil {
			slog.Error("reaper: failed to re-enqueue task",
				"log_id", notifLog.ID,
				"error", err,
			)
			continue
		}

		recovered++
		slog.Info("reaper: recovered stale task",
			"log_id", notifLog.ID,
			"original_status", notifLog.Status,
			"age", time.Since(notifLog.UpdatedAt).Round(time.Second),
		)
	}

	if recovered > 0 {
		slog.Info("reaper: sweep complete", "recovered", recovered, "total_stale", len(staleLogs))
	}
}
