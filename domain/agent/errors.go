package agent

import "errors"

// Domain errors for the agent runtime.
var (
	// ErrInvalidState indicates the state is not a recognized canonical state.
	ErrInvalidState = errors.New("invalid state")

	// ErrInvalidTransition indicates an attempted state transition is not allowed.
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrRunTerminated indicates an operation was attempted on a terminated run.
	ErrRunTerminated = errors.New("run already terminated")

	// ErrRunNotStarted indicates an operation requires a started run.
	ErrRunNotStarted = errors.New("run not started")

	// ErrRunPaused indicates an operation requires an active run.
	ErrRunPaused = errors.New("run is paused")
)
