// Package statemachine provides the statekit integration for the agent runtime.
package statemachine

import (
	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/policy"
)

// Context carries run state through the state machine.
type Context struct {
	Run         *agent.Run
	Budget      *policy.Budget
	Ledger      *ledger.Ledger
	Eligibility *policy.ToolEligibility
	Transitions *policy.StateTransitions
}

// NewContext creates a new machine context.
func NewContext(run *agent.Run, budget *policy.Budget, ledger *ledger.Ledger) *Context {
	return &Context{
		Run:         run,
		Budget:      budget,
		Ledger:      ledger,
		Eligibility: policy.NewToolEligibility(),
		Transitions: policy.DefaultTransitions(),
	}
}

// State IDs as StateID type for statekit.
const (
	stateIntake   statekit.StateID = statekit.StateID(agent.StateIntake)
	stateExplore  statekit.StateID = statekit.StateID(agent.StateExplore)
	stateDecide   statekit.StateID = statekit.StateID(agent.StateDecide)
	stateAct      statekit.StateID = statekit.StateID(agent.StateAct)
	stateValidate statekit.StateID = statekit.StateID(agent.StateValidate)
	stateDone     statekit.StateID = statekit.StateID(agent.StateDone)
	stateFailed   statekit.StateID = statekit.StateID(agent.StateFailed)
)

// NewAgentMachine creates the canonical agent statechart.
func NewAgentMachine() (*statekit.MachineConfig[*Context], error) {
	return statekit.NewMachine[*Context]("agent").
		WithInitial(stateIntake).
		WithContext(&Context{}).
		// Register actions
		WithAction("logEntry", logStateEntry).
		WithAction("recordTransition", recordTransition).
		// Register guards
		WithGuard("canTransition", guardCanTransition).
		WithGuard("budgetAvailable", guardBudgetAvailable).
		// Define states
		State(stateIntake).
			OnEntry("logEntry").
			On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition").
			On("FAIL").Target(stateFailed).Do("recordTransition").
			Done().
		State(stateExplore).
			OnEntry("logEntry").
			On("DECIDE").Target(stateDecide).Guard("canTransition").Do("recordTransition").
			On("FAIL").Target(stateFailed).Do("recordTransition").
			Done().
		State(stateDecide).
			OnEntry("logEntry").
			On("ACT").Target(stateAct).Guard("canTransition").Guard("budgetAvailable").Do("recordTransition").
			On("DONE").Target(stateDone).Do("recordTransition").
			On("FAIL").Target(stateFailed).Do("recordTransition").
			Done().
		State(stateAct).
			OnEntry("logEntry").
			On("VALIDATE").Target(stateValidate).Guard("canTransition").Do("recordTransition").
			On("FAIL").Target(stateFailed).Do("recordTransition").
			Done().
		State(stateValidate).
			OnEntry("logEntry").
			On("DONE").Target(stateDone).Do("recordTransition").
			On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition"). // Allow looping
			On("FAIL").Target(stateFailed).Do("recordTransition").
			Done().
		State(stateDone).
			Final().
			OnEntry("logEntry").
			Done().
		State(stateFailed).
			Final().
			OnEntry("logEntry").
			Done().
		Build()
}

// EventForTransition returns the event type for a state transition.
func EventForTransition(to agent.State) statekit.EventType {
	switch to {
	case agent.StateExplore:
		return "EXPLORE"
	case agent.StateDecide:
		return "DECIDE"
	case agent.StateAct:
		return "ACT"
	case agent.StateValidate:
		return "VALIDATE"
	case agent.StateDone:
		return "DONE"
	case agent.StateFailed:
		return "FAIL"
	default:
		return statekit.EventType(to)
	}
}

// StateFromMachine converts the machine state ID to domain State.
func StateFromMachine(stateID statekit.StateID) agent.State {
	return agent.State(stateID)
}
