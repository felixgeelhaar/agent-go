// Package email provides tools for sending and parsing emails.
package email

import (
	"context"
	"errors"
	"time"
)

// Common errors for email operations.
var (
	ErrProviderNotConfigured = errors.New("email provider not configured")
	ErrInvalidInput          = errors.New("invalid input")
	ErrDeliveryFailed        = errors.New("email delivery failed")
	ErrTemplateNotFound      = errors.New("template not found")
	ErrInvalidTemplate       = errors.New("invalid template")
)

// Provider defines the interface for email providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Send sends an email.
	Send(ctx context.Context, req SendRequest) (SendResponse, error)

	// Parse parses an email from raw content.
	Parse(ctx context.Context, req ParseRequest) (ParseResponse, error)

	// RenderTemplate renders an email template with variables.
	RenderTemplate(ctx context.Context, req RenderTemplateRequest) (RenderTemplateResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// SendRequest represents a request to send an email.
type SendRequest struct {
	// From is the sender email address.
	From string `json:"from"`

	// To are the recipient email addresses.
	To []string `json:"to"`

	// CC are the carbon copy recipients.
	CC []string `json:"cc,omitempty"`

	// BCC are the blind carbon copy recipients.
	BCC []string `json:"bcc,omitempty"`

	// ReplyTo is the reply-to address.
	ReplyTo string `json:"reply_to,omitempty"`

	// Subject is the email subject.
	Subject string `json:"subject"`

	// Body is the email body (plain text).
	Body string `json:"body,omitempty"`

	// HTMLBody is the HTML version of the body.
	HTMLBody string `json:"html_body,omitempty"`

	// Attachments are file attachments.
	Attachments []Attachment `json:"attachments,omitempty"`

	// Headers are custom email headers.
	Headers map[string]string `json:"headers,omitempty"`

	// Priority is the email priority (low, normal, high).
	Priority string `json:"priority,omitempty"`
}

// Attachment represents an email attachment.
type Attachment struct {
	// Filename is the attachment filename.
	Filename string `json:"filename"`

	// ContentType is the MIME type.
	ContentType string `json:"content_type"`

	// Content is the attachment content (base64 encoded).
	Content string `json:"content"`

	// Inline indicates if the attachment is inline.
	Inline bool `json:"inline,omitempty"`

	// ContentID is the Content-ID for inline attachments.
	ContentID string `json:"content_id,omitempty"`
}

// SendResponse represents the result of sending an email.
type SendResponse struct {
	// MessageID is the unique identifier for the sent email.
	MessageID string `json:"message_id"`

	// Success indicates if the email was sent.
	Success bool `json:"success"`

	// Timestamp is when the email was sent.
	Timestamp time.Time `json:"timestamp"`

	// Recipients contains per-recipient delivery status.
	Recipients []RecipientStatus `json:"recipients,omitempty"`
}

// RecipientStatus represents delivery status for a recipient.
type RecipientStatus struct {
	// Email is the recipient email address.
	Email string `json:"email"`

	// Status is the delivery status.
	Status string `json:"status"`

	// Error is any error message.
	Error string `json:"error,omitempty"`
}

// ParseRequest represents a request to parse an email.
type ParseRequest struct {
	// RawContent is the raw email content (RFC 5322 format).
	RawContent string `json:"raw_content"`

	// ExtractAttachments indicates whether to extract attachments.
	ExtractAttachments bool `json:"extract_attachments,omitempty"`
}

// ParseResponse represents a parsed email.
type ParseResponse struct {
	// From is the sender.
	From EmailAddress `json:"from"`

	// To are the recipients.
	To []EmailAddress `json:"to"`

	// CC are the CC recipients.
	CC []EmailAddress `json:"cc,omitempty"`

	// Subject is the email subject.
	Subject string `json:"subject"`

	// Body is the plain text body.
	Body string `json:"body,omitempty"`

	// HTMLBody is the HTML body.
	HTMLBody string `json:"html_body,omitempty"`

	// Date is when the email was sent.
	Date time.Time `json:"date"`

	// MessageID is the email Message-ID header.
	MessageID string `json:"message_id,omitempty"`

	// Headers are the email headers.
	Headers map[string][]string `json:"headers,omitempty"`

	// Attachments are the extracted attachments.
	Attachments []Attachment `json:"attachments,omitempty"`
}

// EmailAddress represents an email address with optional name.
type EmailAddress struct {
	// Name is the display name.
	Name string `json:"name,omitempty"`

	// Address is the email address.
	Address string `json:"address"`
}

// RenderTemplateRequest represents a request to render an email template.
type RenderTemplateRequest struct {
	// TemplateName is the name of the template to render.
	TemplateName string `json:"template_name,omitempty"`

	// TemplateContent is the inline template content.
	TemplateContent string `json:"template_content,omitempty"`

	// Variables are the template variables.
	Variables map[string]interface{} `json:"variables"`

	// Format is the template format (text, html, mjml).
	Format string `json:"format,omitempty"`
}

// RenderTemplateResponse represents a rendered email template.
type RenderTemplateResponse struct {
	// Subject is the rendered subject.
	Subject string `json:"subject,omitempty"`

	// Body is the rendered plain text body.
	Body string `json:"body,omitempty"`

	// HTMLBody is the rendered HTML body.
	HTMLBody string `json:"html_body,omitempty"`

	// Success indicates if rendering succeeded.
	Success bool `json:"success"`
}
