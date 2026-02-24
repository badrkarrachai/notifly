package notification

// Channel represents a notification delivery channel.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"  // future
	ChannelPush  Channel = "push" // future
)

// NotificationType enumerates all supported notification template types.
type NotificationType string

const (
	TypeConfirmSignup    NotificationType = "confirm_signup"
	TypeInviteUser       NotificationType = "invite_user"
	TypeMagicLink        NotificationType = "magic_link"
	TypeChangeEmail      NotificationType = "change_email"
	TypeResetPassword    NotificationType = "reset_password"
	TypeReauthentication NotificationType = "reauthentication"
	TypePasswordChanged  NotificationType = "password_changed"
	TypeEmailChanged     NotificationType = "email_changed"
	TypePhoneChanged     NotificationType = "phone_changed"
	TypeIdentityLinked   NotificationType = "identity_linked"
	TypeIdentityUnlinked NotificationType = "identity_unlinked"
)

// validTypes is the set of all recognized notification types.
var validTypes = map[NotificationType]bool{
	TypeConfirmSignup:    true,
	TypeInviteUser:       true,
	TypeMagicLink:        true,
	TypeChangeEmail:      true,
	TypeResetPassword:    true,
	TypeReauthentication: true,
	TypePasswordChanged:  true,
	TypeEmailChanged:     true,
	TypePhoneChanged:     true,
	TypeIdentityLinked:   true,
	TypeIdentityUnlinked: true,
}

// IsValidType checks whether a notification type is recognized.
func IsValidType(t NotificationType) bool {
	return validTypes[t]
}

// SendRequest is the API request payload for sending a notification.
type SendRequest struct {
	Channel        Channel          `json:"channel" binding:"required,oneof=email sms push"`
	Type           NotificationType `json:"type" binding:"required"`
	To             string           `json:"to" binding:"required"`
	Data           map[string]any   `json:"data"`
	IdempotencyKey string           `json:"idempotency_key"`
}

// SendResponse is the API response payload after a notification is enqueued.
type SendResponse struct {
	ID             string `json:"id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Channel        string `json:"channel"`
	Status         string `json:"status"`
}

// Message is the internal rendered message ready for delivery.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}
