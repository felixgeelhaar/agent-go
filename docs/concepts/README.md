# Core Concepts

agent-go is built on four foundational concepts that work together to create trustworthy AI agents.

## Overview

| Concept | Purpose | Key Benefit |
|---------|---------|-------------|
| [**States**](states.md) | Define where the agent is in its lifecycle | Structural constraints on behavior |
| [**Tools**](tools.md) | Define what the agent can do | Explicit capabilities with metadata |
| [**Planners**](planners.md) | Decide what the agent should do next | Swappable intelligence layer |
| [**Policies**](policies.md) | Enforce limits on agent behavior | Hard constraints LLMs can't bypass |

## How They Work Together

```
                    ┌─────────────┐
                    │   Planner   │ ← Decides next action
                    └──────┬──────┘
                           │
                           ▼
┌──────────┐        ┌─────────────┐        ┌──────────┐
│  Policy  │───────►│   Engine    │◄───────│  Tools   │
│ (limits) │        │ (orchestrates)       │(capabilities)
└──────────┘        └──────┬──────┘        └──────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │   States    │ ← Constrains execution
                    └─────────────┘
```

1. **States** define the current context (explore vs act)
2. **Policies** determine what's allowed in that state
3. **Planners** choose an action from allowed options
4. **Tools** execute the chosen action
5. **Engine** orchestrates the flow and enforces invariants

## Design Philosophy

### Trust Through Structure

agent-go doesn't try to make LLMs behave correctly through prompting. Instead, it creates structural constraints that make incorrect behavior impossible:

- A tool can only run in explicitly allowed states
- Budget limits are enforced in code, not requested in prompts
- Destructive operations require approval regardless of what the LLM says

### Explicit Over Implicit

Every capability is explicitly declared:

```go
// The tool declares its behavior
tool := agent.NewToolBuilder("delete_file").
    WithAnnotations(agent.Annotations{
        Destructive: true,      // Explicitly marked
        RiskLevel:   agent.RiskHigh,
    }).
    MustBuild()

// The policy declares where it can run
eligibility.Allow(agent.StateAct, "delete_file")  // Only in act state

// The engine enforces both
```

### Testable by Design

Real LLMs are unpredictable. agent-go separates the intelligence layer (Planner) from the execution layer (Engine):

```go
// Testing: Use ScriptedPlanner for deterministic behavior
testPlanner := agent.NewScriptedPlanner(steps...)

// Production: Use LLM planner for real intelligence
prodPlanner := planner.NewLLMPlanner(config)

// Same engine, same behavior guarantees
engine, _ := agent.New(
    agent.WithPlanner(testPlanner),  // or prodPlanner
)
```

## Quick Reference

### States

| State | Side Effects | Typical Tools |
|-------|--------------|---------------|
| `intake` | None | None (goal parsing) |
| `explore` | None | Read-only tools |
| `decide` | None | None (planning) |
| `act` | **Yes** | Destructive tools |
| `validate` | None | Read-only tools |
| `done` | None | None (terminal) |
| `failed` | None | None (terminal) |

### Tool Annotations

| Annotation | Effect |
|------------|--------|
| `ReadOnly: true` | Can run in explore/validate |
| `Destructive: true` | Requires approval, only in act |
| `Idempotent: true` | Automatic retry on failure |
| `Cacheable: true` | Results memoized |

### Policy Types

| Policy | Purpose |
|--------|---------|
| Budget | Hard limits on resource usage |
| Approval | Human sign-off for risky operations |
| Eligibility | Per-state tool restrictions |

## Next Steps

Read each concept guide in order:

1. [States](states.md) - Understanding the state machine
2. [Tools](tools.md) - Creating and annotating tools
3. [Planners](planners.md) - Scripted, mock, and LLM planners
4. [Policies](policies.md) - Budgets, approvals, eligibility
