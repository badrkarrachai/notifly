package queue

import (
	"fmt"
	"time"

	"notifly/internal/domain/notification"

	"github.com/hibiken/asynq"
)

// NewClient creates a new asynq client connected to Redis.
func NewClient(redisAddr, password string, db int) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})
}

// NewServer creates a new asynq server connected to Redis.
func NewServer(redisAddr, password string, db int, concurrency int) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: password,
			DB:       db,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"notifications": 10, // priority weight
				"default":       1,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 30s, 60s, 120s, 240s, 480s
				return time.Duration(30*(1<<uint(n-1))) * time.Second
			},
		},
	)
}

// EnqueueSendNotification enqueues a send notification task.
func EnqueueSendNotification(client *asynq.Client, logID string, maxRetry int) error {
	task, err := notification.NewSendNotificationTask(logID)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	_, err = client.Enqueue(task,
		asynq.MaxRetry(maxRetry),
		asynq.Queue("notifications"),
	)
	if err != nil {
		return fmt.Errorf("enqueuing task: %w", err)
	}

	return nil
}
