package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notifly/internal/domain/notification"

	"github.com/supabase-community/postgrest-go"
	supa "github.com/supabase-community/supabase-go"
)

const tableName = "notification_logs"

var _ notification.NotificationStore = (*SupabaseStore)(nil)

// SupabaseStore implements NotificationStore using the Supabase Go SDK.
type SupabaseStore struct {
	client *supa.Client
}

// NewSupabaseStore creates a new Supabase-backed notification store.
func NewSupabaseStore(supabaseURL, serviceKey string) (*SupabaseStore, error) {
	client, err := supa.NewClient(supabaseURL, serviceKey, nil)
	if err != nil {
		return nil, fmt.Errorf("creating supabase client: %w", err)
	}
	return &SupabaseStore{client: client}, nil
}

// supabaseRow is the internal representation for Supabase PostgREST insert/update.
type supabaseRow struct {
	ID             string         `json:"id,omitempty"`
	IdempotencyKey *string        `json:"idempotency_key,omitempty"`
	Channel        string         `json:"channel"`
	Type           string         `json:"type"`
	Recipient      string         `json:"recipient"`
	TemplateData   map[string]any `json:"template_data,omitempty"`
	ProviderID     *string        `json:"provider_id,omitempty"`
	Status         string         `json:"status"`
	ErrorMessage   *string        `json:"error_message,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
	SentAt         *string        `json:"sent_at,omitempty"`
	DeliveredAt    *string        `json:"delivered_at,omitempty"`
	OpenedAt       *string        `json:"opened_at,omitempty"`
	BouncedAt      *string        `json:"bounced_at,omitempty"`
}

// Create inserts a new notification log record.
func (s *SupabaseStore) Create(ctx context.Context, log *notification.NotificationLog) error {
	row := supabaseRow{
		Channel:   log.Channel,
		Type:      log.Type,
		Recipient: log.Recipient,
		Status:    string(log.Status),
	}

	if log.IdempotencyKey != "" {
		row.IdempotencyKey = &log.IdempotencyKey
	}

	if log.TemplateData != nil {
		row.TemplateData = log.TemplateData
	}

	// Insert and get the created row back
	var results []supabaseRow
	data, _, err := s.client.From(tableName).Insert(row, false, "", "representation", "").Execute()
	if err != nil {
		return fmt.Errorf("inserting notification log: %w", err)
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("parsing insert response: %w", err)
	}

	if len(results) > 0 {
		log.ID = results[0].ID
		if results[0].CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, results[0].CreatedAt); err == nil {
				log.CreatedAt = t
			}
		}
		if results[0].UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, results[0].UpdatedAt); err == nil {
				log.UpdatedAt = t
			}
		}
	}

	return nil
}

// GetByID retrieves a notification log by its ID.
func (s *SupabaseStore) GetByID(ctx context.Context, id string) (*notification.NotificationLog, error) {
	data, _, err := s.client.From(tableName).Select("*", "exact", false).Eq("id", id).Single().Execute()
	if err != nil {
		return nil, fmt.Errorf("fetching notification log: %w", err)
	}

	var row supabaseRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, fmt.Errorf("parsing notification log: %w", err)
	}

	return rowToLog(&row), nil
}

// GetByIdempotencyKey retrieves a notification log by its idempotency key.
// Returns nil, nil if no record is found.
func (s *SupabaseStore) GetByIdempotencyKey(ctx context.Context, key string) (*notification.NotificationLog, error) {
	data, _, err := s.client.From(tableName).Select("*", "exact", false).Eq("idempotency_key", key).Execute()
	if err != nil {
		return nil, fmt.Errorf("fetching by idempotency key: %w", err)
	}

	var rows []supabaseRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parsing idempotency result: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	return rowToLog(&rows[0]), nil
}

// UpdateStatus updates the status of a notification log.
func (s *SupabaseStore) UpdateStatus(ctx context.Context, id string, status notification.NotificationStatus, providerID string, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	update := map[string]any{
		"status":     string(status),
		"updated_at": now,
	}

	if providerID != "" {
		update["provider_id"] = providerID
	}

	if errMsg != "" {
		update["error_message"] = errMsg
	}

	switch status {
	case notification.StatusSent:
		update["sent_at"] = now
	case notification.StatusFailed:
		// no extra timestamp
	}

	_, _, err := s.client.From(tableName).Update(update, "", "").Eq("id", id).Execute()
	if err != nil {
		return fmt.Errorf("updating notification status: %w", err)
	}

	return nil
}

// UpdateWebhookStatus updates the status of a notification based on provider ID.
func (s *SupabaseStore) UpdateWebhookStatus(ctx context.Context, providerID string, status notification.NotificationStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	update := map[string]any{
		"status":     string(status),
		"updated_at": now,
	}

	switch status {
	case notification.StatusDelivered:
		update["delivered_at"] = now
	case notification.StatusBounced:
		update["bounced_at"] = now
	case notification.StatusOpened:
		update["opened_at"] = now
	}

	_, _, err := s.client.From(tableName).Update(update, "", "").Eq("provider_id", providerID).Execute()
	if err != nil {
		return fmt.Errorf("updating webhook status: %w", err)
	}

	return nil
}

// List retrieves notification logs with pagination and filtering.
func (s *SupabaseStore) List(ctx context.Context, filter notification.ListFilter) ([]*notification.NotificationLog, int, error) {
	// Apply defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	offset := (filter.Page - 1) * filter.PageSize

	query := s.client.From(tableName).Select("*", "exact", false)

	// Apply filters
	if filter.Status != "" {
		query = query.Eq("status", filter.Status)
	}
	if filter.Recipient != "" {
		query = query.Eq("recipient", filter.Recipient)
	}
	if filter.Channel != "" {
		query = query.Eq("channel", filter.Channel)
	}

	// Order by created_at desc, paginate
	query = query.Order("created_at", &postgrest.OrderOpts{Ascending: false})
	query = query.Range(offset, offset+filter.PageSize-1, "")

	data, count, err := query.Execute()
	if err != nil {
		return nil, 0, fmt.Errorf("listing notification logs: %w", err)
	}

	var rows []supabaseRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, 0, fmt.Errorf("parsing notification list: %w", err)
	}

	logs := make([]*notification.NotificationLog, len(rows))
	for i, row := range rows {
		logs[i] = rowToLog(&row)
	}

	return logs, int(count), nil
}

// ListStale retrieves notification logs stuck in queued/processing for longer than olderThan.
func (s *SupabaseStore) ListStale(ctx context.Context, olderThan time.Time, limit int) ([]*notification.NotificationLog, error) {
	if limit <= 0 {
		limit = 50
	}

	threshold := olderThan.UTC().Format(time.RFC3339Nano)

	// Query for records with status in (queued, processing) AND updated_at < threshold
	query := s.client.From(tableName).
		Select("*", "exact", false).
		In("status", []string{string(notification.StatusQueued), string(notification.StatusProcessing)}).
		Lt("updated_at", threshold).
		Order("updated_at", &postgrest.OrderOpts{Ascending: true}).
		Range(0, limit-1, "")

	data, _, err := query.Execute()
	if err != nil {
		return nil, fmt.Errorf("listing stale notifications: %w", err)
	}

	var rows []supabaseRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parsing stale notifications: %w", err)
	}

	logs := make([]*notification.NotificationLog, len(rows))
	for i, row := range rows {
		logs[i] = rowToLog(&row)
	}

	return logs, nil
}

// rowToLog converts a supabaseRow to a NotificationLog.
func rowToLog(row *supabaseRow) *notification.NotificationLog {
	log := &notification.NotificationLog{
		ID:        row.ID,
		Channel:   row.Channel,
		Type:      row.Type,
		Recipient: row.Recipient,
		Status:    notification.NotificationStatus(row.Status),
	}

	if row.IdempotencyKey != nil {
		log.IdempotencyKey = *row.IdempotencyKey
	}
	if row.TemplateData != nil {
		log.TemplateData = row.TemplateData
	}
	if row.ProviderID != nil {
		log.ProviderID = *row.ProviderID
	}
	if row.ErrorMessage != nil {
		log.ErrorMessage = *row.ErrorMessage
	}

	if row.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err == nil {
			log.CreatedAt = t
		}
	}
	if row.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, row.UpdatedAt); err == nil {
			log.UpdatedAt = t
		}
	}
	if row.SentAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *row.SentAt); err == nil {
			log.SentAt = &t
		}
	}
	if row.DeliveredAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *row.DeliveredAt); err == nil {
			log.DeliveredAt = &t
		}
	}
	if row.OpenedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *row.OpenedAt); err == nil {
			log.OpenedAt = &t
		}
	}
	if row.BouncedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *row.BouncedAt); err == nil {
			log.BouncedAt = &t
		}
	}

	return log
}
