// Package agent provides the core domain model for the agent runtime.
package agent

// State represents a structural constraint in the agent's execution.
// States are identified by stable strings, not behavioral definitions.
type State string

// Canonical states as defined in the TDD.
const (
	StateIntake   State = "intake"   // Normalize goal
	StateExplore  State = "explore"  // Gather evidence
	StateDecide   State = "decide"   // Choose next step
	StateAct      State = "act"      // Perform side-effects
	StateValidate State = "validate" // Confirm outcome
	StateDone     State = "done"     // Terminal success
	StateFailed   State = "failed"   // Terminal failure
)

// IsTerminal returns true if this is a terminal state (done or failed).
func (s State) IsTerminal() bool {
	return s == StateDone || s == StateFailed
}

// AllowsSideEffects returns true if the state permits side-effect operations.
func (s State) AllowsSideEffects() bool {
	return s == StateAct
}

// IsValid returns true if the state is a recognized canonical state.
func (s State) IsValid() bool {
	switch s {
	case StateIntake, StateExplore, StateDecide, StateAct, StateValidate, StateDone, StateFailed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}

// AllStates returns all canonical states.
func AllStates() []State {
	return []State{
		StateIntake,
		StateExplore,
		StateDecide,
		StateAct,
		StateValidate,
		StateDone,
		StateFailed,
	}
}

// TerminalStates returns all terminal states.
func TerminalStates() []State {
	return []State{StateDone, StateFailed}
}

// NonTerminalStates returns all non-terminal states.
func NonTerminalStates() []State {
	return []State{
		StateIntake,
		StateExplore,
		StateDecide,
		StateAct,
		StateValidate,
	}
}
