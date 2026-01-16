package api

import (
	"context"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	domaintool "github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// NewToolBuilder creates a new tool builder.
func NewToolBuilder(name string) *domaintool.Builder {
	return domaintool.NewBuilder(name)
}

// NewToolRegistry creates a new in-memory tool registry.
func NewToolRegistry() *memory.ToolRegistry {
	return memory.NewToolRegistry()
}

// NewKnowledgeStore creates a new in-memory knowledge store for vector embeddings.
// If dimension is 0, it will be auto-detected from the first vector stored.
func NewKnowledgeStore(dimension int) *memory.KnowledgeStore {
	return memory.NewKnowledgeStore(dimension)
}

// NewMockPlanner creates a mock planner with predefined decisions.
func NewMockPlanner(decisions ...Decision) *planner.MockPlanner {
	return planner.NewMockPlanner(decisions...)
}

// NewScriptedPlanner creates a scripted planner for deterministic testing.
func NewScriptedPlanner(steps ...planner.ScriptStep) *planner.ScriptedPlanner {
	return planner.NewScriptedPlanner(steps...)
}

// ScriptStep is a step in a scripted planner.
type ScriptStep = planner.ScriptStep

// NewToolEligibility creates a new tool eligibility configuration.
func NewToolEligibility() *policy.ToolEligibility {
	return policy.NewToolEligibility()
}

// NewStateTransitions creates a new state transitions configuration.
func NewStateTransitions() *policy.StateTransitions {
	return policy.NewStateTransitions()
}

// DefaultTransitions returns the canonical state transition configuration.
func DefaultTransitions() *policy.StateTransitions {
	return policy.DefaultTransitions()
}

// NewAutoApprover creates an approver that automatically approves all requests.
func NewAutoApprover(name string) *policy.AutoApprover {
	return policy.NewAutoApprover(name)
}

// NewDenyApprover creates an approver that automatically denies all requests.
func NewDenyApprover(reason string) *policy.DenyApprover {
	return policy.NewDenyApprover(reason)
}

// Decision constructors

// NewCallToolDecision creates a decision to execute a tool.
func NewCallToolDecision(toolName string, input []byte, reason string) Decision {
	return agent.NewCallToolDecision(toolName, input, reason)
}

// NewTransitionDecision creates a decision to transition states.
func NewTransitionDecision(toState State, reason string) Decision {
	return agent.NewTransitionDecision(toState, reason)
}

// NewFinishDecision creates a decision to complete successfully.
func NewFinishDecision(summary string, result []byte) Decision {
	return agent.NewFinishDecision(summary, result)
}

// NewFailDecision creates a decision to terminate with failure.
func NewFailDecision(reason string, err error) Decision {
	return agent.NewFailDecision(reason, err)
}

// AutoApprover returns an approver that automatically approves all requests.
// This is a convenience function for development and testing.
func AutoApprover() policy.Approver {
	return policy.NewAutoApprover("auto")
}

// DenyApprover returns an approver that automatically denies all requests.
// This is a convenience function for testing rejection scenarios.
func DenyApprover(reason string) policy.Approver {
	return policy.NewDenyApprover(reason)
}

// ApprovalRequest is re-exported for callback approvers.
type ApprovalRequest = policy.ApprovalRequest

// ApprovalResponse is re-exported for callback approvers.
type ApprovalResponse = policy.ApprovalResponse

// CallbackApprover implements the Approver interface using a callback function.
type CallbackApprover struct {
	callback func(ctx context.Context, req ApprovalRequest) (bool, error)
}

// NewCallbackApprover creates an approver that uses a callback function for decisions.
func NewCallbackApprover(fn func(ctx context.Context, req ApprovalRequest) (bool, error)) *CallbackApprover {
	return &CallbackApprover{callback: fn}
}

// Approve processes the approval request using the callback function.
func (c *CallbackApprover) Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	approved, err := c.callback(ctx, req)
	if err != nil {
		return ApprovalResponse{}, err
	}
	return ApprovalResponse{
		Approved:  approved,
		Approver:  "callback",
		Timestamp: time.Now(),
	}, nil
}
