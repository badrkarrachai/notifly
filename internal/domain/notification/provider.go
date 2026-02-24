package notification

import "context"

// Provider defines the contract for a notification delivery channel.
// Implementations live in infra/ (e.g., Resend for email, Twilio for SMS).
type Provider interface {
	// Send delivers a rendered message and returns the provider's message ID.
	Send(ctx context.Context, msg *Message) (string, error)

	// Channel returns which delivery channel this provider handles.
	Channel() Channel
}

// TemplateRenderer defines the contract for rendering notification templates.
// Implementations live in infra/template/.
type TemplateRenderer interface {
	// Render produces a subject line, HTML body, and plain-text body for the given notification type.
	Render(notifType NotificationType, data map[string]any) (subject, html, text string, err error)
}
