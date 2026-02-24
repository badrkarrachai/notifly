package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"notifly/internal/domain/notification"
)

var _ notification.Provider = (*ResendProvider)(nil)

// ResendProvider sends emails using the Resend API.
type ResendProvider struct {
	apiKey      string
	fromAddress string
	fromName    string
	httpClient  *http.Client
}

// NewResendProvider creates a new Resend email provider.
func NewResendProvider(apiKey, fromAddress, fromName string) *ResendProvider {
	return &ResendProvider{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Channel returns the email channel identifier.
func (p *ResendProvider) Channel() notification.Channel {
	return notification.ChannelEmail
}

// Send delivers an email via the Resend API and returns the message ID.
func (p *ResendProvider) Send(ctx context.Context, msg *notification.Message) (string, error) {
	from := p.fromAddress
	if p.fromName != "" {
		from = fmt.Sprintf("%s <%s>", p.fromName, p.fromAddress)
	}

	payload := map[string]any{
		"from":    from,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"html":    msg.HTML,
	}

	// Include plain-text version if available
	if msg.Text != "" {
		payload["text"] = msg.Text
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message    string `json:"message"`
			StatusCode int    `json:"statusCode"`
		}
		_ = json.Unmarshal(respBody, &errResp)

		msg := errResp.Message
		if msg == "" {
			msg = fmt.Sprintf("resend API error: status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("resend: %s", msg)
	}

	var successResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &successResp); err != nil {
		return "", fmt.Errorf("parsing resend response: %w", err)
	}

	return successResp.ID, nil
}
