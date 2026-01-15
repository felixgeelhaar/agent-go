package statemachine

import (
	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// logStateEntry logs when entering a state.
// In statekit, actions receive a pointer to the context. Since our context is *Context,
// actions receive **Context.
func logStateEntry(ctx **Context, event statekit.Event) {
	if ctx == nil || *ctx == nil || (*ctx).Run == nil {
		return
	}

	c := *ctx

	// Get target state from payload if available
	var newState agent.State
	if payload, ok := event.Payload.(TransitionPayload); ok {
		newState = payload.ToState
	} else {
		// Derive from event type
		newState = stateFromEventType(event.Type)
	}

	if newState != "" {
		c.Run.CurrentState = newState
	}
}

// recordTransition records the state transition in the ledger.
func recordTransition(ctx **Context, event statekit.Event) {
	if ctx == nil || *ctx == nil || (*ctx).Run == nil || (*ctx).Ledger == nil {
		return
	}

	c := *ctx
	fromState := c.Run.CurrentState

	// Get target state and reason from payload
	var toState agent.State
	var reason string
	if payload, ok := event.Payload.(TransitionPayload); ok {
		toState = payload.ToState
		reason = payload.Reason
	} else {
		// Derive from event type
		toState = stateFromEventType(event.Type)
	}

	c.Ledger.RecordTransition(fromState, toState, reason)

	// Update run state
	c.Run.TransitionTo(toState)
}

// ActionWithReason creates a payload that includes a reason in the event.
func ActionWithReason(reason string) TransitionPayload {
	return TransitionPayload{
		Reason: reason,
	}
}
