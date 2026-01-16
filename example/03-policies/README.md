# 03 - Policies

Demonstrates policy enforcement: budgets that limit resource consumption and approvals that require human sign-off.

## What This Example Shows

- Budget enforcement with hard limits
- Interactive approval workflow for destructive tools
- How policies stop agents regardless of planner decisions
- The difference between agent failure (policy) and execution error

## Run It

```bash
go run main.go
```

## Expected Output

```
=== Example 1: Budget Enforcement ===
Setting budget to 3 tool calls...

  [count tool] Counter is now: 1
  [count tool] Counter is now: 2
  [count tool] Counter is now: 3

Run status: failed
Failure reason: budget exhausted: tool_calls
Steps taken: 4 (stopped at budget limit)

=== Example 2: Approval Workflow ===
Dangerous tools require human approval...

  [APPROVAL REQUIRED]
  Tool: danger
  Risk Level: high
  Input: {"action":"delete_everything"}
  Approve? (y/n): n
  -> Denied!

Run status: failed
Failure reason: approval denied for tool: danger
```

## Budget Enforcement

Budgets are **hard limits** that cannot be bypassed:

```go
engine, _ := agent.New(
    agent.WithBudget("tool_calls", 3),  // Maximum 3 tool calls
)
```

When the budget is exhausted:
1. The engine stops executing
2. The run status becomes `failed`
3. The failure reason explains why

### Multiple Budget Types

```go
agent.WithBudget("tool_calls", 100),    // Overall tool usage
agent.WithBudget("api_calls", 50),       // External API calls
agent.WithBudget("tokens", 10000),       // LLM token usage
```

## Approval Workflow

Approvals require human confirmation before executing risky operations:

```go
// Create an approver
approver := agent.NewCallbackApprover(func(ctx context.Context, req ApprovalRequest) (bool, error) {
    // Present to user, get decision
    return userApproved, nil
})

engine, _ := agent.New(
    agent.WithApprover(approver),
)
```

### What Triggers Approval

By default:
- Tools with `Destructive: true`
- Tools with `RiskLevel: RiskHigh` or `RiskCritical`

### Auto-Approval for Testing

```go
// Auto-approve everything (testing only!)
agent.WithApprover(agent.AutoApprover())

// Auto-deny everything
agent.WithApprover(agent.DenyAllApprover())
```

## Key Concepts

### Policies vs Planner Decisions

| Aspect | Planner | Policy |
|--------|---------|--------|
| Who decides | LLM / Script | Engine |
| Can be overridden | No | No |
| Purpose | Intelligence | Safety |

### Why This Matters

Without policies:
```
LLM: "I'll just make 10,000 API calls to be thorough!"
Agent: *makes 10,000 API calls*
User: *gets huge bill*
```

With policies:
```
LLM: "I'll just make 10,000 API calls to be thorough!"
Policy: Budget exhausted after 100 calls
Agent: *stops safely*
User: *bill is predictable*
```

## Next Steps

- **[04-llm-planner](../04-llm-planner/)** - Use a real LLM for planning
- **[05-observability](../05-observability/)** - Add tracing and metrics
