# 01 - Minimal Agent

The simplest possible agent-go example. Demonstrates the absolute minimum code needed for a working agent.

## What This Example Shows

- Creating a basic tool with `NewToolBuilder`
- Using `ScriptedPlanner` for deterministic behavior
- Building and running an engine
- Checking run results

## Run It

```bash
go run main.go
```

## Expected Output

```
=== Minimal Agent Example ===
Status: completed
Result: {"status":"complete"}
```

## Code Walkthrough

### 1. Create a Tool

```go
echoTool := agent.NewToolBuilder("echo").
    WithDescription("Echoes the input message").
    WithAnnotations(agent.Annotations{ReadOnly: true}).
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        // Parse input, do work, return result
    }).
    MustBuild()
```

### 2. Create a Planner

```go
planner := agent.NewScriptedPlanner(
    agent.ScriptStep{ExpectState: agent.StateIntake, Decision: ...},
    agent.ScriptStep{ExpectState: agent.StateExplore, Decision: ...},
    agent.ScriptStep{ExpectState: agent.StateExplore, Decision: ...},
)
```

### 3. Allow the tool in explore (explicit allow-list)

```go
eligibility := agent.NewToolEligibility()
eligibility.Allow(agent.StateExplore, "echo")
```

Omitting eligibility also works: the engine defaults to
`NewDefaultToolEligibility()` (wildcard allow in explore/decide/act/validate).
This example uses an explicit allow-list to show the production pattern.

### 4. Build Engine

```go
engine, err := agent.New(
    agent.WithTool(echoTool),
    agent.WithPlanner(planner),
    agent.WithToolEligibility(eligibility),
)
```

### 5. Run

```go
run, err := engine.Run(context.Background(), "Echo a message")
```

## Next Steps

- **[02-tools](../02-tools/)** - Learn to create multiple tools with different annotations
- **[03-policies](../03-policies/)** - Add budgets and approval requirements
