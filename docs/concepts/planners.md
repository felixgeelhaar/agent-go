# Planners

Planners are the "brain" of your agent - they decide what action to take next. agent-go separates planning from execution, making it easy to swap intelligence layers without changing your agent's structure.

## The Planner Interface

All planners implement a simple interface:

```go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Decision, error)
}

type PlanRequest struct {
    RunID        string
    CurrentState State
    Evidence     []Evidence
    AllowedTools []string
    Budgets      BudgetSnapshot
    Vars         map[string]any
}
```

Import via `agent "go.klarlabs.de/agent/interfaces/api"` — `Planner` and `PlanRequest` are re-exported there.

## Decision Types

Planners return one of five decision types:

### CallTool

```go
decision := agent.NewCallToolDecision(
    "read_file",
    json.RawMessage(`{"path": "/tmp/x"}`),
    "gathering information",
)
```

### Transition

```go
decision := agent.NewTransitionDecision(
    agent.StateAct,
    "ready to make changes",
)
```

### Finish

Complete successfully. Under `DefaultTransitions`, **Finish is only valid from `decide` or `validate`** (those states may transition to `done`).

```go
decision := agent.NewFinishDecision(
    "task completed",
    json.RawMessage(`{"result": "success"}`),
)
```

### Fail

```go
decision := agent.NewFailDecision(
    "cannot proceed without API key",
    nil, // optional underlying error
)
```

### AskHuman

Pause for human input, then resume with `engine.ResumeWithInput`:

```go
decision := agent.NewAskHumanDecision(
    "Should I delete these 100 files?",
    "yes", "no", "review",
)
```

## Built-in Planners

### ScriptedPlanner

For deterministic testing. Follows a predefined script:

```go
planner := agent.NewScriptedPlanner(
    agent.ScriptStep{
        ExpectState: agent.StateIntake,
        Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    agent.NewCallToolDecision("read_file", input, "reading"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    agent.NewTransitionDecision(agent.StateDecide, "ready"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateDecide,
        Decision:    agent.NewFinishDecision("done", result),
    },
)
```

**Use for**: Unit tests, integration tests, demos

### MockPlanner

Returns decisions in order (or a single fixed decision):

```go
planner := agent.NewMockPlanner(
    agent.NewTransitionDecision(agent.StateExplore, "start"),
    agent.NewTransitionDecision(agent.StateDecide, "decide"),
    agent.NewFinishDecision("immediate finish", nil),
)
```

**Use for**: Simple tests, edge case testing

### RuleBasedPlanner / HybridPlanner

Go-native rules evaluated in priority order; hybrid adds any fallback planner (for example an LLM):

```go
rules := agent.NewRuleBasedPlanner(
    agent.NewFailDecision("no rule matched", nil),
    // rules built with agent.NewRule(...)
)

hybrid := agent.NewHybridPlanner(rules, llmFallback)
```

**Use for**: Deterministic business rules, with optional LLM fallback

## LLM Planners

For production agents, use the contrib LLM planner module:

```go
import (
    plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
    "go.klarlabs.de/agent/contrib/planner-llm/providers"
)

provider := providers.NewAnthropicProvider(providers.AnthropicConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-sonnet-4-20250514",
})

llmPlanner := plannerllm.NewPlanner(plannerllm.Config{
    Provider:    provider,
    Temperature: 0.7,
    MaxTokens:   4096,
})

engine, _ := agent.New(agent.WithPlanner(llmPlanner), /* tools... */)
```

Other providers live under `contrib/planner-llm/providers` (OpenAI, Gemini, Ollama, Bedrock, Cohere, Copilot). See `example/04-llm-planner` for a runnable walkthrough.

## Creating Custom Planners

Implement `agent.Planner`:

```go
type MyPlanner struct {
    // ...
}

func (p *MyPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    if req.CurrentState == agent.StateIntake {
        return agent.NewTransitionDecision(agent.StateExplore, "starting"), nil
    }
    if len(req.Evidence) > 5 {
        return agent.NewTransitionDecision(agent.StateDecide, "enough info"), nil
    }
    if req.CurrentState == agent.StateDecide {
        return agent.NewFinishDecision("done", nil), nil
    }
    return agent.NewCallToolDecision("gather_more", nil, "need more"), nil
}
```

### Composite Planner

```go
type CompositeP struct {
    primary  agent.Planner
    fallback agent.Planner
}

func (p *CompositeP) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    decision, err := p.primary.Plan(ctx, req)
    if err != nil {
        return p.fallback.Plan(ctx, req)
    }
    return decision, nil
}
```

Or use `agent.NewHybridPlanner` when the primary path is rule-based.

## Planner Guarantees

Regardless of implementation, planners must satisfy these guarantees:

### 1. Bounded Output

Decisions are finite and well-defined. A planner cannot return arbitrary actions.

### 2. No Side Effects

Planners only analyze and decide. They never execute. The engine handles execution.

### 3. Conservative Bias

When uncertain, planners should prefer safe options (read over write, wait over act).

### 4. Deterministic Mode

For testing, planners must support deterministic behavior (e.g., ScriptedPlanner).

## Best Practices

### 1. Test with ScriptedPlanner

```go
func TestAgentBehavior(t *testing.T) {
    planner := agent.NewScriptedPlanner(expectedSteps...)

    engine, err := agent.New(
        agent.WithPlanner(planner),
        agent.WithTool(readTool),
        agent.WithTool(writeTool),
    )
    require.NoError(t, err)

    run, err := engine.Run(ctx, "test goal")
    require.NoError(t, err)
    assert.Equal(t, agent.StatusCompleted, run.Status)
}
```

### 2. Separate Planning from Execution

Don't let planners access tools or I/O directly:

```go
// Good - planner only decides
func (p *MyPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    return agent.NewCallToolDecision("read_file", input, "need info"), nil
}

// Bad - planner executes (violates separation)
func (p *BadPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    content, _ := os.ReadFile(path) // Don't do this!
    return agent.NewFinishDecision("done", content), nil
}
```

### 3. Include Reasoning

Always provide a reason for decisions:

```go
// Good - explains why
agent.NewCallToolDecision("delete_file", input, "file is temporary and task complete")

// Bad - no context
agent.NewCallToolDecision("delete_file", input, "")
```

### 4. Respect Allowed Tools and Budgets

Only choose tools from `req.AllowedTools`, and check `req.Budgets` before long tool loops.

## Next Steps

- [Policies](policies.md) - Budgets, approvals, eligibility
- [Tools](tools.md) - Creating tools with annotations
- [States](states.md) - The canonical state machine
