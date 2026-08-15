# Quick Start Guide

Get your first agent running in 5 minutes.

## Prerequisites

- **Go 1.26.2 or later** (module floor; see `CONTRIBUTING.md`)
- A code editor

## Step 1: Create a New Project

```bash
mkdir my-first-agent
cd my-first-agent
go mod init my-first-agent
go get go.klarlabs.de/agent/interfaces/api@latest
```

## Step 2: Create Your Agent

Create `main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    agent "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/domain/tool"
)

func main() {
    // Create a simple calculator tool
    addTool := agent.NewToolBuilder("add").
        WithDescription("Adds two numbers").
        WithAnnotations(agent.Annotations{
            ReadOnly:   true,
            Idempotent: true,
        }).
        WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
            var in struct {
                A float64 `json:"a"`
                B float64 `json:"b"`
            }
            if err := json.Unmarshal(input, &in); err != nil {
                return tool.Result{}, err
            }

            result := in.A + in.B
            output, _ := json.Marshal(map[string]float64{"result": result})
            return tool.Result{Output: output}, nil
        }).
        MustBuild()

    // Create a scripted planner that defines the agent's behavior.
    // Finish is only allowed from decide (or validate) under DefaultTransitions.
    planner := agent.NewScriptedPlanner(
        // Step 1: Move from intake to explore
        agent.ScriptStep{
            ExpectState: agent.StateIntake,
            Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting calculation"),
        },
        // Step 2: Call the add tool
        agent.ScriptStep{
            ExpectState: agent.StateExplore,
            Decision:    agent.NewCallToolDecision("add", json.RawMessage(`{"a": 5, "b": 3}`), "adding numbers"),
        },
        // Step 3: Move to decide
        agent.ScriptStep{
            ExpectState: agent.StateExplore,
            Decision:    agent.NewTransitionDecision(agent.StateDecide, "ready to finish"),
        },
        // Step 4: Finish with the result
        agent.ScriptStep{
            ExpectState: agent.StateDecide,
            Decision:    agent.NewFinishDecision("calculation complete", json.RawMessage(`{"answer": 8}`)),
        },
    )

    // Build the engine.
    // Default eligibility allows registered tools in explore/decide/act/validate.
    // For production, prefer an explicit allow-list via WithToolEligibility.
    engine, err := agent.New(
        agent.WithTool(addTool),
        agent.WithPlanner(planner),
        agent.WithMaxSteps(10),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Run the agent
    run, err := engine.Run(context.Background(), "Calculate 5 + 3")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("=== Agent Run Complete ===")
    fmt.Printf("Status: %s\n", run.Status)
    fmt.Printf("Result: %s\n", run.Result)
    fmt.Printf("Final state: %s\n", run.CurrentState)
}
```

## Step 3: Run Your Agent

```bash
go run .
```

Expected output:
```
=== Agent Run Complete ===
Status: completed
Result: {"answer": 8}
Final state: done
```

## What Just Happened?

1. **Tool Creation**: We created an `add` tool with annotations indicating it's read-only and idempotent
2. **Planner Setup**: We used a `ScriptedPlanner` which follows a predetermined script (perfect for testing)
3. **Engine Configuration**: We built the engine with our tool and planner (default eligibility is enough to start)
4. **Execution**: The engine ran through the scripted steps: intake → explore → decide → done

## Understanding the State Machine

The agent moved through these states:

```
intake (start)
   ↓
   └─→ "starting calculation"

explore (gathering info)
   ↓
   └─→ called "add" tool with {a: 5, b: 3}
   └─→ "ready to finish"

decide
   ↓
   └─→ Finish → done (terminal)
```

## Next Steps

### Tighten Tool Eligibility (recommended for production)

```go
eligibility := agent.NewToolEligibilityWith(agent.EligibilityRules{
    agent.StateExplore: {"add"},
})

engine, _ := agent.New(
    agent.WithTool(addTool),
    agent.WithPlanner(planner),
    agent.WithToolEligibility(eligibility),
)
```

### Add More Tools

```go
multiplyTool := agent.NewToolBuilder("multiply").
    WithDescription("Multiplies two numbers").
    WithAnnotations(agent.Annotations{ReadOnly: true}).
    WithHandler(multiplyHandler).
    MustBuild()

engine, _ := agent.New(
    agent.WithTool(addTool),
    agent.WithTool(multiplyTool),
    // ...
)
```

### Add Budget Limits

```go
engine, _ := agent.New(
    agent.WithTool(addTool),
    agent.WithPlanner(planner),
    agent.WithBudget("tool_calls", 10),  // Max 10 tool calls
    agent.WithMaxSteps(20),               // Max 20 total steps
)
```

### Use a Real LLM Planner

See [`example/04-llm-planner`](../example/04-llm-planner/) and the
[`contrib/planner-llm`](../contrib/planner-llm/) module for provider wiring.

### Explore Examples

```bash
go run ./example/01-minimal
go run ./example/flagship
```

## Learn More

- **[Concepts: States](concepts/states.md)** - Understanding the state machine
- **[Concepts: Tools](concepts/tools.md)** - Creating powerful tools
- **[Concepts: Planners](concepts/planners.md)** - From scripted to LLM-powered
- **[Concepts: Policies](concepts/policies.md)** - Budgets, approvals, eligibility
- **[Examples](../example/)** - Progressive examples from minimal to production
