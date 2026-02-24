package template

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"notifly/internal/domain/notification"
)

var _ notification.TemplateRenderer = (*Engine)(nil)

// templateMeta holds the subject and template name mapping for each notification type.
type templateMeta struct {
	Subject      string
	TemplateName string
}

// registry maps notification types to their metadata.
var registry = map[notification.NotificationType]templateMeta{
	notification.TypeConfirmSignup:    {Subject: "Confirm Your Email Address", TemplateName: "confirm_signup"},
	notification.TypeInviteUser:       {Subject: "You've Been Invited", TemplateName: "invite_user"},
	notification.TypeMagicLink:        {Subject: "Your Sign-In Link", TemplateName: "magic_link"},
	notification.TypeChangeEmail:      {Subject: "Confirm Your New Email Address", TemplateName: "change_email"},
	notification.TypeResetPassword:    {Subject: "Reset Your Password", TemplateName: "reset_password"},
	notification.TypeReauthentication: {Subject: "Confirm Your Identity", TemplateName: "reauthentication"},
	notification.TypePasswordChanged:  {Subject: "Your Password Has Been Changed", TemplateName: "password_changed"},
	notification.TypeEmailChanged:     {Subject: "Your Email Address Has Been Changed", TemplateName: "email_changed"},
	notification.TypePhoneChanged:     {Subject: "Your Phone Number Has Been Changed", TemplateName: "phone_changed"},
	notification.TypeIdentityLinked:   {Subject: "A New Identity Has Been Linked", TemplateName: "identity_linked"},
	notification.TypeIdentityUnlinked: {Subject: "An Identity Has Been Unlinked", TemplateName: "identity_unlinked"},
}

// Engine renders notification templates using Go's html/template package.
type Engine struct {
	templates *template.Template
}

// NewEngine creates a new template engine by loading all templates from the given directory.
func NewEngine(templatesDir string) (*Engine, error) {
	tmpl, err := template.ParseGlob(templatesDir + "/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates from %s: %w", templatesDir, err)
	}

	return &Engine{templates: tmpl}, nil
}

// Render produces a subject line, HTML body, and plain-text fallback for the given notification type.
func (e *Engine) Render(notifType notification.NotificationType, data map[string]any) (subject, html, text string, err error) {
	meta, ok := registry[notifType]
	if !ok {
		return "", "", "", fmt.Errorf("no template registered for type: %s", notifType)
	}

	// Allow subject override via data
	subject = meta.Subject
	if customSubject, ok := data["Subject"].(string); ok && customSubject != "" {
		subject = customSubject
	}

	// Render the HTML template
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, meta.TemplateName+".html", data); err != nil {
		return "", "", "", fmt.Errorf("executing template %s: %w", meta.TemplateName, err)
	}
	html = buf.String()

	// Generate plain-text fallback by stripping HTML tags
	text = stripHTML(html)

	return subject, html, text, nil
}

// stripHTML removes HTML tags and collapses whitespace to produce a plain-text version.
func stripHTML(s string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(s, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Collapse whitespace
	wsRe := regexp.MustCompile(`\s+`)
	text = wsRe.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
