package policy

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// ToolEligibility defines which tools are allowed in which states.
type ToolEligibility struct {
	allowed map[agent.State]map[string]bool
}

// NewToolEligibility creates a new tool eligibility configuration.
func NewToolEligibility() *ToolEligibility {
	return &ToolEligibility{
		allowed: make(map[agent.State]map[string]bool),
	}
}

// Allow permits a tool in the given state.
func (e *ToolEligibility) Allow(state agent.State, toolName string) *ToolEligibility {
	if e.allowed[state] == nil {
		e.allowed[state] = make(map[string]bool)
	}
	e.allowed[state][toolName] = true
	return e
}

// AllowMultiple permits multiple tools in the given state.
func (e *ToolEligibility) AllowMultiple(state agent.State, toolNames ...string) *ToolEligibility {
	for _, name := range toolNames {
		e.Allow(state, name)
	}
	return e
}

// IsAllowed checks if a tool is allowed in the given state.
func (e *ToolEligibility) IsAllowed(state agent.State, toolName string) bool {
	stateTools, exists := e.allowed[state]
	if !exists {
		return false
	}
	return stateTools[toolName]
}

// AllowedTools returns all tools allowed in the given state.
func (e *ToolEligibility) AllowedTools(state agent.State) []string {
	stateTools, exists := e.allowed[state]
	if !exists {
		return nil
	}

	tools := make([]string, 0, len(stateTools))
	for name := range stateTools {
		tools = append(tools, name)
	}
	return tools
}

// StateTransitions defines allowed state transitions.
type StateTransitions struct {
	transitions map[agent.State][]agent.State
}

// NewStateTransitions creates a new state transition configuration.
func NewStateTransitions() *StateTransitions {
	return &StateTransitions{
		transitions: make(map[agent.State][]agent.State),
	}
}

// Allow permits a transition from one state to another.
func (t *StateTransitions) Allow(from, to agent.State) *StateTransitions {
	t.transitions[from] = append(t.transitions[from], to)
	return t
}

// CanTransition checks if a transition is allowed.
func (t *StateTransitions) CanTransition(from, to agent.State) bool {
	allowed, exists := t.transitions[from]
	if !exists {
		return false
	}

	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}

// AllowedTransitions returns all states reachable from the given state.
func (t *StateTransitions) AllowedTransitions(from agent.State) []agent.State {
	return t.transitions[from]
}

// DefaultTransitions returns the canonical state transition configuration.
func DefaultTransitions() *StateTransitions {
	return NewStateTransitions().
		Allow(agent.StateIntake, agent.StateExplore).
		Allow(agent.StateIntake, agent.StateFailed).
		Allow(agent.StateExplore, agent.StateDecide).
		Allow(agent.StateExplore, agent.StateFailed).
		Allow(agent.StateDecide, agent.StateAct).
		Allow(agent.StateDecide, agent.StateDone).
		Allow(agent.StateDecide, agent.StateFailed).
		Allow(agent.StateAct, agent.StateValidate).
		Allow(agent.StateAct, agent.StateFailed).
		Allow(agent.StateValidate, agent.StateDone).
		Allow(agent.StateValidate, agent.StateExplore). // Allow looping back
		Allow(agent.StateValidate, agent.StateFailed)
}

// Constraint is a generic policy constraint that can be evaluated.
type Constraint interface {
	// Evaluate checks if the constraint is satisfied.
	Evaluate(ctx ConstraintContext) (bool, string)
}

// ConstraintContext provides context for constraint evaluation.
type ConstraintContext struct {
	RunID        string
	CurrentState agent.State
	ToolName     string
	Budget       *Budget
}
