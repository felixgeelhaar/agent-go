// Package notification provides tools for sending notifications.
package notification

import (
	"context"
	"errors"
)

// Common errors for notification operations.
var (
	ErrProviderNotConfigured = errors.New("notification provider not configured")
	ErrInvalidInput          = errors.New("invalid input")
	ErrChannelNotFound       = errors.New("channel not found")
	ErrSendFailed            = errors.New("failed to send notification")
	ErrProviderUnavailable   = errors.New("provider unavailable")
)

// Provider defines the interface for notification providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Send sends a notification.
	Send(ctx context.Context, req SendRequest) (SendResponse, error)

	// Update updates an existing notification (if supported).
	Update(ctx context.Context, req UpdateRequest) (UpdateResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// SendRequest represents a notification send request.
type SendRequest struct {
	// Channel is the target channel or recipient.
	Channel string `json:"channel"`

	// Message is the notification content.
	Message string `json:"message"`

	// Title is an optional notification title.
	Title string `json:"title,omitempty"`

	// Level is the notification level (info, warning, error, success).
	Level string `json:"level,omitempty"`

	// Attachments are optional file attachments.
	Attachments []Attachment `json:"attachments,omitempty"`

	// Metadata is provider-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// ThreadID is for threaded messages (if supported).
	ThreadID string `json:"thread_id,omitempty"`
}

// Attachment represents a file attachment.
type Attachment struct {
	// Name is the attachment filename.
	Name string `json:"name"`

	// Content is the attachment content (base64 encoded).
	Content string `json:"content"`

	// ContentType is the MIME type.
	ContentType string `json:"content_type"`
}

// SendResponse represents the result of sending a notification.
type SendResponse struct {
	// MessageID is the sent message identifier.
	MessageID string `json:"message_id"`

	// Timestamp is when the message was sent.
	Timestamp string `json:"timestamp"`

	// Channel is the channel where the message was sent.
	Channel string `json:"channel"`

	// Success indicates if the send was successful.
	Success bool `json:"success"`
}

// UpdateRequest represents a notification update request.
type UpdateRequest struct {
	// MessageID is the message to update.
	MessageID string `json:"message_id"`

	// Channel is the channel containing the message.
	Channel string `json:"channel"`

	// Message is the updated content.
	Message string `json:"message,omitempty"`

	// Title is the updated title.
	Title string `json:"title,omitempty"`

	// Metadata is provider-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateResponse represents the result of updating a notification.
type UpdateResponse struct {
	// MessageID is the updated message identifier.
	MessageID string `json:"message_id"`

	// Updated indicates if the update was successful.
	Updated bool `json:"updated"`

	// Timestamp is when the message was updated.
	Timestamp string `json:"timestamp"`
}
