package run

import "errors"

// Domain errors for run store operations.
var (
	// ErrRunNotFound is returned when a run does not exist.
	ErrRunNotFound = errors.New("run not found")

	// ErrRunExists is returned when attempting to create a run that already exists.
	ErrRunExists = errors.New("run already exists")

	// ErrInvalidRunID is returned when a run ID is invalid (e.g., empty).
	ErrInvalidRunID = errors.New("invalid run ID")

	// ErrConcurrentUpdate is returned when a concurrent update conflict is detected.
	ErrConcurrentUpdate = errors.New("concurrent update conflict")

	// ErrConnectionFailed is returned when connection to the store backend fails.
	ErrConnectionFailed = errors.New("store connection failed")

	// ErrOperationTimeout is returned when a store operation times out.
	ErrOperationTimeout = errors.New("store operation timeout")
)
