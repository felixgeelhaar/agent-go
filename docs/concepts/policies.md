# Policies

Policies are **hard constraints** that the runtime enforces regardless of what the planner decides. They're the guardrails that make agents trustworthy.

## Why Policies Matter

LLMs can be creative in unexpected ways. Policies ensure that creativity stays within safe bounds:

```
Without Policies:
  LLM: "I'll just delete all the files to clean up!"
  Agent: *deletes everything*

With Policies:
  LLM: "I'll just delete all the files to clean up!"
  Policy: Budget exhausted after 10 tool calls
  Agent: *stops safely*
```

## Types of Policies

agent-go provides three policy mechanisms:

| Policy | Purpose | Enforcement |
|--------|---------|-------------|
| **Budgets** | Limit resource consumption | Hard limits, no exceptions |
| **Approvals** | Human sign-off for risky operations | Block until approved |
| **Eligibility** | Control tool access per state | Tools can't run where not allowed |

## Budgets

Budgets set hard limits on resource consumption.

### Basic Budget

```go
engine, _ := agent.New(
    agent.WithBudget("tool_calls", 100),  // Max 100 tool calls
    agent.WithBudget("tokens", 50000),     // Named budget (consume via middleware)
)
```

The engine automatically consumes the `tool_calls` budget on each successful tool execution. Other named budgets are available to planners via `PlanRequest.Budgets` and can be decremented with `BudgetFromContextMiddleware` in the middleware chain.

### How Budgets Work

1. Each run starts with the configured budget
2. Every successful tool call decrements `tool_calls` (when configured)
3. When a budget cannot cover the next call, the engine stops
4. Planners see remaining budgets in `PlanRequest.Budgets`

### Budget Exhaustion

When a budget is exhausted:

```go
run, err := engine.Run(ctx, "Process all files")
// err is policy.ErrBudgetExceeded (wrapped)
// run.Status == agent.StatusFailed
```

### Budget Visibility

Planners can see remaining budgets:

```go
func (p *MyPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    remaining := req.Budgets["tool_calls"]
    if remaining < 5 {
        // Running low, wrap up — Finish is valid from decide/validate
        return agent.NewFinishDecision("completing due to budget", result), nil
    }
    // Continue normal operation
}
```

## Approvals

Approvals require human sign-off before executing risky operations.

### Setting Up Approval

```go
approver := agent.NewCallbackApprover(func(ctx context.Context, req agent.ApprovalRequest) (bool, error) {
    fmt.Printf("Approve %s with input %s? [y/n]: ", req.ToolName, req.Input)
    var response string
    fmt.Scanln(&response)
    return response == "y", nil
})

engine, _ := agent.New(
    agent.WithApprover(approver),
)
```

### What Triggers Approval

By default (via tool annotations), approval is required when a tool's annotations say so — typically:

1. Tools marked `Destructive: true`
2. Tools with high / critical risk levels (`ShouldRequireApproval()`)

### Approval Request Structure

```go
type ApprovalRequest struct {
    RunID     string
    ToolName  string
    Input     json.RawMessage
    Reason    string
    RiskLevel string
    Timestamp time.Time
}
```

### Auto-Approval for Testing

```go
// In tests, auto-approve everything
engine, _ := agent.New(
    agent.WithApprover(agent.AutoApprover()),
)

// Or auto-deny everything
engine, _ := agent.New(
    agent.WithApprover(agent.DenyApprover("denied in test")),
)
```

## Tool Eligibility

Eligibility controls which tools are available in each state.

### Basic Eligibility

```go
eligibility := agent.NewToolEligibility()

// Read-only tools in explore and validate
eligibility.AllowMultiple(agent.StateExplore, "read_file", "list_dir", "search")
eligibility.AllowMultiple(agent.StateValidate, "read_file", "verify_result")

// Destructive tools only in act
eligibility.AllowMultiple(agent.StateAct, "write_file", "delete_file", "execute")

engine, _ := agent.New(
    agent.WithToolEligibility(eligibility),
)
```

Declarative form:

```go
eligibility := agent.NewToolEligibilityWith(agent.EligibilityRules{
    agent.StateExplore:  {"read_file", "list_dir"},
    agent.StateAct:      {"write_file", "delete_file"},
    agent.StateValidate: {"read_file"},
})
```

### How Eligibility Works

1. When a planner requests a tool call, the engine checks eligibility
2. If the tool isn't allowed in the current state, the call is rejected
3. The planner sees only allowed tools in `PlanRequest.AllowedTools`

### Denied Tool Calls

If a planner tries to call an ineligible tool:

```go
// Planner requests write_file in explore state
decision := agent.NewCallToolDecision("write_file", input, "writing")

// Engine rejects it — run ends failed
// run.Status == agent.StatusFailed
// error / message mentions the tool is not allowed in the current state
```

### Default Eligibility

If you omit `WithToolEligibility`, the engine uses `NewDefaultToolEligibility()`:

```go
// Wildcard allow in explore / decide / act / validate.
// Intake and terminal states still deny tools.
eligibility := agent.NewDefaultToolEligibility()
```

Prefer an explicit allow-list in production. Pass `agent.NewToolEligibility()` (empty) for deny-by-default.

## Combining Policies

Policies work together to create layered protection:

```go
engine, _ := agent.New(
    // Layer 1: Hard budget limits
    agent.WithBudget("tool_calls", 50),

    // Layer 2: State-based tool restrictions
    agent.WithToolEligibility(eligibility),

    // Layer 3: Human approval for dangerous operations
    agent.WithApprover(approver),
)
```

### Enforcement Order

1. **Structural act-gate**: Side-effecting tools only in states that allow side effects (`act`)
2. **Eligibility check**: Is the tool allowed in this state?
3. **Governance / budget**: Can the call proceed under remaining budget?
4. **Approval**: Does this tool require approval?
5. **Execution**: Tool runs only if all checks pass

## Policy Events

Watch policy outcomes through the event stream (requires `WithEventStore`):

```go
import (
    "go.klarlabs.de/agent/domain/event"
    "go.klarlabs.de/agent/infrastructure/storage/memory"
)

store := memory.NewEventStore()
engine, _ := agent.New(
    agent.WithPlanner(planner),
    agent.WithEventStore(store),
    // ...
)

runID, events, _ := engine.Stream(ctx, "Process files")
for evt := range events {
    switch evt.Type {
    case event.TypeBudgetExhausted:
        log.Warn("budget exhausted", "run", runID)
    case event.TypeApprovalDenied:
        log.Warn("approval denied", "run", runID)
    case event.TypeToolFailed:
        log.Warn("tool failed", "run", runID, "payload", evt.Payload)
    }
}
```

## Best Practices

### 1. Start Conservative

Begin with tight limits and loosen as needed:

```go
agent.WithBudget("tool_calls", 10),
// Expand after testing
agent.WithBudget("tool_calls", 100),
```

### 2. Prefer Explicit Eligibility

```go
eligibility := agent.NewToolEligibilityWith(agent.EligibilityRules{
    agent.StateExplore:  {"read_file", "list_dir"},
    agent.StateAct:      {"write_file"},
    agent.StateValidate: {"read_file"},
})
```

### 3. Separate by Risk

Different tools deserve different treatment:

```go
eligibility.AllowMultiple(agent.StateExplore, lowRiskTools...)
eligibility.AllowMultiple(agent.StateValidate, lowRiskTools...)
eligibility.AllowMultiple(agent.StateAct, lowRiskTools...)
eligibility.AllowMultiple(agent.StateAct, highRiskTools...)
// High-risk / destructive tools still go through approval via annotations
```

### 4. Observe via Events

Prefer `Stream` / `EventStore` over ad-hoc hooks so policy decisions land in the same audit trail as the rest of the run.

### 5. Test Policy Boundaries

```go
func TestBudgetEnforcement(t *testing.T) {
    engine, _ := agent.New(
        agent.WithBudget("tool_calls", 2),
        agent.WithPlanner(plannerThatCallsToolsForever),
        agent.WithTool(readTool),
    )

    run, err := engine.Run(ctx, "test")

    assert.ErrorIs(t, err, policy.ErrBudgetExceeded)
    assert.Equal(t, agent.StatusFailed, run.Status)
    assert.Equal(t, 2, run.ConsumedToolCalls())
}
```

## Next Steps

- [States](states.md) - The canonical state machine
- [Tools](tools.md) - Annotations that drive approval and risk
- [Planners](planners.md) - Decisions that policies constrain
