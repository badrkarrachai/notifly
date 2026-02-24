package notification

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// TaskTypeSendNotification is the asynq task type for sending notifications.
const TaskTypeSendNotification = "notification:send"

// SendNotificationPayload is the serialized payload for a send notification task.
type SendNotificationPayload struct {
	LogID string `json:"log_id"`
}

// NewSendNotificationTask creates a new asynq task for sending a notification.
func NewSendNotificationTask(logID string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendNotificationPayload{LogID: logID})
	if err != nil {
		return nil, fmt.Errorf("marshaling task payload: %w", err)
	}
	return asynq.NewTask(TaskTypeSendNotification, payload), nil
}

// ParseSendNotificationPayload deserializes the task payload.
func ParseSendNotificationPayload(data []byte) (*SendNotificationPayload, error) {
	var p SendNotificationPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshaling task payload: %w", err)
	}
	return &p, nil
}
