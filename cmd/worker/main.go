package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"notifly/internal/config"
	"notifly/internal/domain/notification"
	"notifly/internal/infra/email"
	"notifly/internal/infra/queue"
	"notifly/internal/infra/store"
	"notifly/internal/infra/template"

	"github.com/hibiken/asynq"
)

// queueEnqueuer adapts the asynq client to the notification.Enqueuer interface.
// Used by the reaper to re-enqueue stale tasks.
type queueEnqueuer struct {
	client   *asynq.Client
	maxRetry int
}

func (q *queueEnqueuer) EnqueueSendNotification(logID string) error {
	return queue.EnqueueSendNotification(q.client, logID, q.maxRetry)
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("worker configuration loaded")

	// ==========================================
	// Dependency Injection (Manual Wiring)
	// ==========================================

	// Resolve the templates directory
	templatesDir := resolveTemplatesDir()

	// Template Engine
	tmplEngine, err := template.NewEngine(templatesDir)
	if err != nil {
		slog.Error("failed to initialize template engine", "error", err, "dir", templatesDir)
		os.Exit(1)
	}
	slog.Info("template engine initialized", "dir", templatesDir)

	// Email Provider (Resend)
	emailProvider := email.NewResendProvider(
		cfg.Email.APIKey,
		cfg.Email.FromAddress,
		cfg.Email.FromName,
	)

	// Supabase Store
	notifStore, err := store.NewSupabaseStore(cfg.Supabase.URL, cfg.Supabase.ServiceKey)
	if err != nil {
		slog.Error("failed to initialize supabase store", "error", err)
		os.Exit(1)
	}
	slog.Info("supabase store initialized")

	// Notification Worker
	notifWorker := notification.NewWorker(notifStore, tmplEngine, emailProvider)

	// Asynq Client (for reaper re-enqueuing)
	asynqClient := queue.NewClient(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB)
	defer asynqClient.Close()

	enqueuer := &queueEnqueuer{
		client:   asynqClient,
		maxRetry: cfg.Queue.MaxRetry,
	}

	// ==========================================
	// Asynq Server (task processing)
	// ==========================================

	asynqServer := queue.NewServer(
		cfg.Redis.Address,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Queue.Concurrency,
	)

	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(notification.TaskTypeSendNotification, func(ctx context.Context, task *asynq.Task) error {
		payload, err := notification.ParseSendNotificationPayload(task.Payload())
		if err != nil {
			return err
		}
		return notifWorker.ProcessTask(ctx, payload.LogID)
	})

	// Start the asynq worker in a goroutine
	go func() {
		slog.Info("worker starting",
			"concurrency", cfg.Queue.Concurrency,
			"redis", cfg.Redis.Address,
		)
		if err := asynqServer.Run(mux); err != nil {
			slog.Error("worker failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// ==========================================
	// Stale Task Reaper
	// ==========================================

	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	defer reaperCancel()

	reaper := notification.NewReaper(notifStore, enqueuer, notification.ReaperConfig{
		Interval:       time.Duration(cfg.Reaper.IntervalSec) * time.Second,
		StaleThreshold: time.Duration(cfg.Reaper.StaleThresholdSec) * time.Second,
		BatchSize:      cfg.Reaper.BatchSize,
	})

	go reaper.Run(reaperCtx)

	// ==========================================
	// Graceful Shutdown
	// ==========================================

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down worker...")
	reaperCancel() // Stop the reaper first
	asynqServer.Shutdown()
	slog.Info("worker exited gracefully")
}

// resolveTemplatesDir finds the templates directory.
func resolveTemplatesDir() string {
	// Check if running in Docker (production)
	if _, err := os.Stat("/app/templates"); err == nil {
		return "/app/templates"
	}

	// Development: resolve relative to the source file location
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "internal/infra/template/templates"
	}

	// Navigate from cmd/worker/main.go to internal/infra/template/templates
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return filepath.Join(projectRoot, "internal", "infra", "template", "templates")
}
