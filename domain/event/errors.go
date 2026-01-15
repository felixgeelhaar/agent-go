package event

import "errors"

// Domain errors for event store operations.
var (
	// ErrEventNotFound is returned when an event does not exist.
	ErrEventNotFound = errors.New("event not found")

	// ErrRunNotFound is returned when events for a run do not exist.
	ErrRunNotFound = errors.New("run not found in event store")

	// ErrSequenceConflict is returned when event sequence numbers conflict.
	ErrSequenceConflict = errors.New("event sequence conflict")

	// ErrInvalidEvent is returned when an event is malformed.
	ErrInvalidEvent = errors.New("invalid event")

	// ErrSnapshotNotFound is returned when no snapshot exists for a run.
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrConnectionFailed is returned when connection to the store backend fails.
	ErrConnectionFailed = errors.New("event store connection failed")

	// ErrOperationTimeout is returned when a store operation times out.
	ErrOperationTimeout = errors.New("event store operation timeout")

	// ErrSubscriptionClosed is returned when a subscription channel is closed.
	ErrSubscriptionClosed = errors.New("event subscription closed")
)
