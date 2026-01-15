package planner

import (
	"context"
	"errors"
	"sync"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// ScriptStep defines an expected state and the decision to return.
type ScriptStep struct {
	// ExpectState asserts we're in this state before returning the decision.
	ExpectState agent.State

	// Decision is the decision to return.
	Decision agent.Decision

	// Condition is an optional additional condition that must be true.
	Condition func(PlanRequest) bool
}

// ScriptedPlanner executes a predefined sequence for deterministic testing.
// It validates that the agent is in the expected state before returning decisions.
type ScriptedPlanner struct {
	steps        []ScriptStep
	index        int
	onUnexpected func(PlanRequest) agent.Decision
	mu           sync.Mutex
}

// NewScriptedPlanner creates a scripted planner with the given steps.
func NewScriptedPlanner(steps ...ScriptStep) *ScriptedPlanner {
	return &ScriptedPlanner{
		steps: steps,
		index: 0,
		onUnexpected: func(_ PlanRequest) agent.Decision {
			return agent.NewFailDecision("unexpected state", errors.New("script exhausted"))
		},
	}
}

// OnUnexpected sets the handler for unexpected states.
func (p *ScriptedPlanner) OnUnexpected(handler func(PlanRequest) agent.Decision) *ScriptedPlanner {
	p.onUnexpected = handler
	return p
}

// Plan returns the next decision if the state matches expectations.
func (p *ScriptedPlanner) Plan(_ context.Context, req PlanRequest) (agent.Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.index >= len(p.steps) {
		return p.onUnexpected(req), nil
	}

	step := p.steps[p.index]

	// Validate expected state
	if step.ExpectState != "" && step.ExpectState != req.CurrentState {
		return agent.Decision{}, &UnexpectedStateError{
			Expected: step.ExpectState,
			Actual:   req.CurrentState,
			StepIndex: p.index,
		}
	}

	// Validate optional condition
	if step.Condition != nil && !step.Condition(req) {
		return agent.Decision{}, &ConditionFailedError{
			StepIndex: p.index,
			State:     req.CurrentState,
		}
	}

	p.index++
	return step.Decision, nil
}

// Reset resets the planner to the beginning.
func (p *ScriptedPlanner) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.index = 0
}

// CurrentStep returns the current step index.
func (p *ScriptedPlanner) CurrentStep() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.index
}

// IsComplete returns true if all steps have been executed.
func (p *ScriptedPlanner) IsComplete() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.index >= len(p.steps)
}

// UnexpectedStateError indicates the planner received an unexpected state.
type UnexpectedStateError struct {
	Expected  agent.State
	Actual    agent.State
	StepIndex int
}

func (e *UnexpectedStateError) Error() string {
	return "unexpected state at step " + string(rune(e.StepIndex)) + ": expected " + string(e.Expected) + ", got " + string(e.Actual)
}

// ConditionFailedError indicates a step condition was not met.
type ConditionFailedError struct {
	StepIndex int
	State     agent.State
}

func (e *ConditionFailedError) Error() string {
	return "condition failed at step " + string(rune(e.StepIndex)) + " in state " + string(e.State)
}
