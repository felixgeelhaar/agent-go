# States

The state machine is the foundation of agent-go's safety model. Every agent operates within a **canonical 7-state lifecycle** that constrains when different types of operations can occur.

## The Canonical States

```
┌─────────┐
│  intake │ ──────────────────────────────┐
└────┬────┘                               │
     │                                    │
     ▼                                    │
┌─────────┐                               │
│ explore │ ◄─────────────────────────────┤
└────┬────┘                               │
     │                                    │
     ▼                                    │
┌─────────┐     ┌──────┐                  │
│ decide  │ ───►│ done │                  │
└────┬────┘     └──────┘                  │
     │               ▲                    │
     ▼               │                    │
┌─────────┐          │                    │
│   act   │ ─────────┤                    │
└────┬────┘          │                    │
     │               │                    │
     ▼               │                    │
┌──────────┐         │      ┌────────┐    │
│ validate │ ────────┴─────►│ failed │◄───┘
└──────────┘                └────────┘
```

## State Semantics

Each state has explicit semantics that the runtime enforces:

| State | Purpose | Side Effects | Allowed Tools |
|-------|---------|--------------|---------------|
| `intake` | Parse and normalize the goal | None | None |
| `explore` | Gather information | None | Read-only only |
| `decide` | Evaluate options, plan | None | None |
| `act` | Execute side-effecting operations | **Yes** | All eligible |
| `validate` | Verify results | None | Read-only only |
| `done` | Successful completion | None | None (terminal) |
| `failed` | Terminal failure | None | None (terminal) |

## Why State Semantics Matter

### Problem: Uncontrolled Side Effects

Without state constraints, an LLM might decide to delete files while "exploring" the filesystem:

```
User: "What files are in my project?"
LLM: *calls delete_file* "I cleaned up some unnecessary files for you!"
```

### Solution: Structural Constraints

With agent-go, this is **impossible**:

```go
// Tool is marked as destructive
deleteFile := agent.NewToolBuilder("delete_file").
    WithAnnotations(agent.Annotations{Destructive: true}).
    Build()

// Destructive tools can only run in 'act' state
eligibility.Allow(agent.StateAct, "delete_file")

// In 'explore' state, the runtime blocks the call
// No amount of LLM persuasion can bypass this
```

## Working with States

### Accessing Current State

```go
run, _ := engine.Run(ctx, "Do something")
fmt.Println(run.CurrentState)  // e.g., "done"
```

### State Transitions

States transition based on planner decisions or engine logic:

```go
// Planner can request a transition
decision := agent.NewTransitionDecision(agent.StateExplore, "need more info")

// Invalid Transition/Finish/Fail targets are soft-rejected: state is unchanged,
// PlanRequest.Feedback explains the allowed targets, and a
// state.transition.rejected event is published. Loop detection bounds retries.
```

### Valid Transitions (`DefaultTransitions`)

| From | To |
|------|----|
| `intake` | `explore`, `failed` |
| `explore` | `decide`, `failed` |
| `decide` | `act`, `done`, `failed` |
| `act` | `validate`, `failed` |
| `validate` | `done`, `explore`, `failed` |

`Finish` requires a path to `done` (from `decide` or `validate`). `Fail` requires a path to `failed`.

### Custom Transitions

```go
transitions := agent.NewStateTransitionsWith(agent.TransitionRules{
    agent.StateIntake:   {agent.StateExplore, agent.StateFailed},
    agent.StateExplore:  {agent.StateDecide, agent.StateAct, agent.StateFailed}, // skip decide
    agent.StateDecide:   {agent.StateAct, agent.StateDone, agent.StateFailed},
    agent.StateAct:      {agent.StateValidate, agent.StateFailed},
    agent.StateValidate: {agent.StateDone, agent.StateFailed},
})

engine, _ := agent.New(
    agent.WithTransitions(transitions),
)
```

## State in Tool Eligibility

The most common use of states is controlling tool access:

```go
eligibility := agent.NewToolEligibility()

// Read-only tools: explore and validate
eligibility.AllowMultiple(agent.StateExplore, "read_file", "list_dir", "search")
eligibility.AllowMultiple(agent.StateValidate, "read_file", "check_result")

// Destructive tools: act only
eligibility.AllowMultiple(agent.StateAct, "write_file", "delete_file", "execute")

engine, _ := agent.New(
    agent.WithToolEligibility(eligibility),
)
```

## Best Practices

### 1. Keep Explore Read-Only

Never allow side-effecting tools in `explore`:

```go
// Good
eligibility.Allow(agent.StateExplore, "read_file")

// Bad - don't do this
eligibility.Allow(agent.StateExplore, "write_file")  // Dangerous!
```

### 2. Use Validate for Verification

After `act`, use `validate` to confirm the operation succeeded:

```go
planner := agent.NewScriptedPlanner(
    // ...
    agent.ScriptStep{
        ExpectState: agent.StateAct,
        Decision:    agent.NewCallToolDecision("write_file", input, "writing"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateAct,
        Decision:    agent.NewTransitionDecision(agent.StateValidate, "verify"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateValidate,
        Decision:    agent.NewCallToolDecision("read_file", input, "checking"),
    },
    // ...
)
```

### 3. Handle Terminal States

Always handle `done` and `failed` appropriately:

```go
run, err := engine.Run(ctx, goal)
if err != nil {
    // Execution error (distinct from agent failure)
    log.Fatal(err)
}

switch run.Status {
case agent.StatusCompleted:
    fmt.Println("Success:", run.Result)
case agent.StatusFailed:
    fmt.Println("Agent failed:", run.Error)
}
```

## Next Steps

- [Tools](tools.md) - Creating tools with state-aware annotations
- [Policies](policies.md) - Combining states with policy enforcement
