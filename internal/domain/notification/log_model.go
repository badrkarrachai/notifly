package notification

import "time"

// NotificationStatus represents the delivery status of a notification.
type NotificationStatus string

const (
	StatusQueued     NotificationStatus = "queued"
	StatusProcessing NotificationStatus = "processing"
	StatusSent       NotificationStatus = "sent"
	StatusFailed     NotificationStatus = "failed"
	StatusDelivered  NotificationStatus = "delivered"
	StatusBounced    NotificationStatus = "bounced"
	StatusOpened     NotificationStatus = "opened"
)

// NotificationLog represents a persisted notification record.
type NotificationLog struct {
	ID             string             `json:"id"`
	IdempotencyKey string             `json:"idempotency_key,omitempty"`
	Channel        string             `json:"channel"`
	Type           string             `json:"type"`
	Recipient      string             `json:"recipient"`
	TemplateData   map[string]any     `json:"template_data,omitempty"`
	ProviderID     string             `json:"provider_id,omitempty"`
	Status         NotificationStatus `json:"status"`
	ErrorMessage   string             `json:"error_message,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	SentAt         *time.Time         `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time         `json:"delivered_at,omitempty"`
	OpenedAt       *time.Time         `json:"opened_at,omitempty"`
	BouncedAt      *time.Time         `json:"bounced_at,omitempty"`
}

// ListFilter defines pagination and filtering options for listing notification logs.
type ListFilter struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Status    string `form:"status"`
	Recipient string `form:"recipient"`
	Channel   string `form:"channel"`
}

// ListResponse wraps a paginated list of notification logs.
type ListResponse struct {
	Notifications []*NotificationLog `json:"notifications"`
	Total         int                `json:"total"`
	Page          int                `json:"page"`
	PageSize      int                `json:"page_size"`
}
