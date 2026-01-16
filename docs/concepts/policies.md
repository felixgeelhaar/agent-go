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
    agent.WithBudget("tokens", 50000),     // Max 50k tokens
)
```

### How Budgets Work

1. Each run starts with the configured budget
2. Every tool call decrements the relevant budget
3. When a budget hits zero, the engine stops
4. Planners see remaining budgets in `PlanRequest.Budgets`

### Budget Exhaustion

When a budget is exhausted:

```go
run, err := engine.Run(ctx, "Process all files")
// err == nil, but:
// run.Status == agent.StatusFailed
// run.FailureReason == "budget exhausted: tool_calls"
```

### Custom Budget Types

Define domain-specific budgets:

```go
engine, _ := agent.New(
    agent.WithBudget("api_calls", 50),      // Limit API requests
    agent.WithBudget("database_queries", 20), // Limit DB access
    agent.WithBudget("llm_tokens", 10000),   // Limit LLM usage
)

// Decrement in tool handler
func apiHandler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    budget := agent.BudgetFromContext(ctx)
    budget.Decrement("api_calls", 1)
    // ...
}
```

### Budget Visibility

Planners can see remaining budgets:

```go
func (p *MyPlanner) Plan(ctx context.Context, req PlanRequest) (Decision, error) {
    remaining := req.Budgets["tool_calls"]
    if remaining < 5 {
        // Running low, wrap up
        return NewFinishDecision("completing due to budget", result), nil
    }
    // Continue normal operation
}
```

## Approvals

Approvals require human sign-off before executing risky operations.

### Setting Up Approval

```go
// Create an approver
approver := agent.NewCallbackApprover(func(ctx context.Context, req ApprovalRequest) (bool, error) {
    // Show to user
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

By default, approval is required for:

1. Tools marked `Destructive: true`
2. Tools with `RiskLevel: RiskHigh` or `RiskLevel: RiskCritical`

### Custom Approval Rules

```go
approver := agent.NewConditionalApprover(
    // Always approve read-only tools
    agent.ApproveIf(func(req ApprovalRequest) bool {
        return req.Tool.Annotations().ReadOnly
    }),
    // Require approval for specific tools
    agent.RequireApprovalFor("delete_file", "drop_table", "send_email"),
    // Require approval above risk threshold
    agent.RequireApprovalAboveRisk(agent.RiskMedium),
)
```

### Approval Request Structure

```go
type ApprovalRequest struct {
    RunID    string          // Current run
    Tool     tool.Tool       // Tool being called
    Input    json.RawMessage // Input to the tool
    State    agent.State     // Current state
    Evidence []Evidence      // Accumulated evidence
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
    agent.WithApprover(agent.DenyAllApprover()),
)
```

## Tool Eligibility

Eligibility controls which tools are available in each state.

### Basic Eligibility

```go
eligibility := agent.NewToolEligibility()

// Read-only tools in explore and validate
eligibility.Allow(agent.StateExplore, "read_file", "list_dir", "search")
eligibility.Allow(agent.StateValidate, "read_file", "verify_result")

// Destructive tools only in act
eligibility.Allow(agent.StateAct, "write_file", "delete_file", "execute")

engine, _ := agent.New(
    agent.WithToolEligibility(eligibility),
)
```

### How Eligibility Works

1. When a planner requests a tool call, the engine checks eligibility
2. If the tool isn't allowed in the current state, the call is rejected
3. The planner sees only allowed tools in `PlanRequest.AllowedTools`

### Denied Tool Calls

If a planner tries to call an ineligible tool:

```go
// Planner requests write_file in explore state
decision := NewCallToolDecision("write_file", input, "writing")

// Engine rejects it
// run.FailureReason == "tool write_file not eligible in state explore"
```

### Default Eligibility

If no eligibility is configured, the engine uses sensible defaults:

```go
// Default: ReadOnly tools in explore/validate, all tools in act
defaultEligibility := agent.DefaultToolEligibility()
```

### Dynamic Eligibility

Eligibility can depend on runtime context:

```go
eligibility := agent.NewDynamicEligibility(func(state agent.State, tool tool.Tool) bool {
    // Allow all tools if user is admin
    if isAdmin := agent.VarFromContext(ctx, "is_admin"); isAdmin == true {
        return true
    }

    // Normal eligibility rules
    if state == agent.StateExplore {
        return tool.Annotations().ReadOnly
    }
    return true
})
```

## Combining Policies

Policies work together to create layered protection:

```go
engine, _ := agent.New(
    // Layer 1: Hard budget limits
    agent.WithBudget("tool_calls", 50),
    agent.WithBudget("tokens", 10000),

    // Layer 2: State-based tool restrictions
    agent.WithToolEligibility(eligibility),

    // Layer 3: Human approval for dangerous operations
    agent.WithApprover(approver),
)
```

### Enforcement Order

1. **Eligibility Check**: Is the tool allowed in this state?
2. **Budget Check**: Is there remaining budget?
3. **Approval Check**: Does this tool require approval?
4. **Execution**: Tool runs only if all checks pass

## Policy Events

Monitor policy enforcement:

```go
engine, _ := agent.New(
    agent.WithPolicyEventHandler(func(event PolicyEvent) {
        switch event.Type {
        case PolicyEventBudgetExhausted:
            log.Warn("Budget exhausted", "budget", event.Budget)
        case PolicyEventApprovalDenied:
            log.Warn("Approval denied", "tool", event.Tool)
        case PolicyEventEligibilityDenied:
            log.Warn("Tool not eligible", "tool", event.Tool, "state", event.State)
        }
    }),
)
```

## Best Practices

### 1. Start Conservative

Begin with tight limits and loosen as needed:

```go
// Start tight
agent.WithBudget("tool_calls", 10),

// Expand after testing
agent.WithBudget("tool_calls", 100),
```

### 2. Use Multiple Budget Types

Don't rely on a single budget:

```go
// Multiple safeguards
agent.WithBudget("tool_calls", 100),    // Overall limit
agent.WithBudget("write_ops", 10),       // Limit writes specifically
agent.WithBudget("api_calls", 50),       // Limit external calls
```

### 3. Separate by Risk

Different tools deserve different treatment:

```go
// Low risk: broadly available
eligibility.Allow(agent.StateExplore, lowRiskTools...)
eligibility.Allow(agent.StateValidate, lowRiskTools...)
eligibility.Allow(agent.StateAct, lowRiskTools...)

// High risk: only in act with approval
eligibility.Allow(agent.StateAct, highRiskTools...)
// Plus approval requirement via annotations
```

### 4. Log Policy Decisions

Make policy enforcement visible:

```go
agent.WithPolicyEventHandler(func(event PolicyEvent) {
    logger.Info("policy decision",
        "type", event.Type,
        "tool", event.Tool,
        "state", event.State,
        "allowed", event.Allowed,
    )
})
```

### 5. Test Policy Boundaries

Verify policies work as expected:

```go
func TestBudgetEnforcement(t *testing.T) {
    engine, _ := agent.New(
        agent.WithBudget("tool_calls", 2),
        agent.WithPlanner(plannerThatCallsToolsForever),
    )

    run, _ := engine.Run(ctx, "test")

    assert.Equal(t, agent.StatusFailed, run.Status)
    assert.Contains(t, run.FailureReason, "budget exhausted")
    assert.Equal(t, 2, run.StepCount)  // Stopped at limit
}
```

## Next Steps

- [Evidence](evidence.md) - How agents accumulate knowledge
- [Ledger](ledger.md) - Audit trail for all operations
