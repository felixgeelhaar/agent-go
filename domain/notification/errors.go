package notification

import "errors"

// Domain errors for notification operations.
var (
	// ErrEndpointUnavailable indicates the webhook endpoint is not reachable.
	ErrEndpointUnavailable = errors.New("webhook endpoint unavailable")

	// ErrEndpointRejected indicates the endpoint rejected the notification.
	ErrEndpointRejected = errors.New("webhook endpoint rejected notification")

	// ErrNotifierClosed indicates the notifier has been closed.
	ErrNotifierClosed = errors.New("notifier is closed")

	// ErrInvalidEndpoint indicates the endpoint configuration is invalid.
	ErrInvalidEndpoint = errors.New("invalid endpoint configuration")

	// ErrBatchTooLarge indicates the batch exceeds the maximum size.
	ErrBatchTooLarge = errors.New("batch exceeds maximum size")

	// ErrEventFilteredOut indicates the event was filtered out.
	ErrEventFilteredOut = errors.New("event filtered out")

	// ErrSigningFailed indicates payload signing failed.
	ErrSigningFailed = errors.New("payload signing failed")
)
