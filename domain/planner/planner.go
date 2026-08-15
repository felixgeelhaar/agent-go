package planner

import (
	"context"
	"encoding/json"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
)

// ToolDef describes a tool available for planning decisions.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// PlanRequest contains all information needed for planning.
type PlanRequest struct {
	RunID        string
	Goal         string
	CurrentState agent.State
	Evidence     []agent.Evidence
	AllowedTools []string
	// AllowedTransitions lists the states reachable from CurrentState under the
	// transition policy. Planners should only emit a transition to one of these
	// (analogous to AllowedTools).
	AllowedTransitions []agent.State
	ToolDefs           []ToolDef
	Budgets            policy.BudgetSnapshot
	Vars               map[string]any
	// Feedback is a one-shot note from the engine about the previous step —
	// e.g. that a transition was rejected — so the planner can self-correct.
	Feedback string
}

// Planner is the decision engine contract. Implementations live in
// infrastructure/planner or external packages; the public API depends only
// on this domain interface so callers never need infrastructure imports.
type Planner interface {
	Plan(ctx context.Context, req PlanRequest) (agent.Decision, error)
}
