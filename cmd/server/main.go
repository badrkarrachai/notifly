package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notifly/internal/config"
	"notifly/internal/domain/notification"
	"notifly/internal/infra/queue"
	"notifly/internal/infra/ratelimit"
	"notifly/internal/infra/store"
	"notifly/internal/router"

	"github.com/hibiken/asynq"
)

// queueEnqueuer adapts the asynq client to the notification.Enqueuer interface.
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

	slog.Info("configuration loaded", "port", cfg.Server.Port, "mode", cfg.Server.Mode)

	// ==========================================
	// Dependency Injection (Manual Wiring)
	// ==========================================

	// Supabase Store
	notifStore, err := store.NewSupabaseStore(cfg.Supabase.URL, cfg.Supabase.ServiceKey)
	if err != nil {
		slog.Error("failed to initialize supabase store", "error", err)
		os.Exit(1)
	}
	slog.Info("supabase store initialized")

	// Asynq Client (for enqueuing tasks)
	asynqClient := queue.NewClient(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB)
	defer asynqClient.Close()
	slog.Info("asynq client initialized", "redis", cfg.Redis.Address)

	// Recipient Rate Limiter
	recipientLimiter := ratelimit.NewRedisRecipientLimiter(
		cfg.Redis.Address,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.RecipientRateLimit.MaxPerHour,
	)
	defer recipientLimiter.Close()
	slog.Info("recipient rate limiter initialized", "max_per_hour", cfg.RecipientRateLimit.MaxPerHour)

	// Enqueuer adapter
	enqueuer := &queueEnqueuer{
		client:   asynqClient,
		maxRetry: cfg.Queue.MaxRetry,
	}

	// Service
	notificationService := notification.NewService(notifStore, enqueuer, recipientLimiter)

	// Handler
	notificationHandler := notification.NewHandler(notificationService)

	// Router
	r := router.New(cfg, notificationHandler)

	// ==========================================
	// HTTP Server with Graceful Shutdown
	// ==========================================

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server starting", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Give outstanding requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}
