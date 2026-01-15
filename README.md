# agent-go

A **state-driven agent runtime** built with Domain-Driven Design (DDD) principles in Go. This framework provides a robust foundation for building AI agents with explicit state management, tool orchestration, and policy enforcement.

## Features

### Core Runtime
- **State Machine Core**: 7-state agent lifecycle (intake → explore → decide → act → validate → done/failed)
- **Tool Orchestration**: Registry-based tool management with schema validation
- **Policy Enforcement**: Budget limits, approval workflows, and tool eligibility rules
- **Resilience Patterns**: Circuit breaker, retry, and bulkhead via [fortify](https://github.com/felixgeelhaar/fortify)
- **Structured Logging**: High-performance logging via [bolt](https://github.com/felixgeelhaar/bolt)
- **Audit Trail**: Complete ledger of all decisions, tool calls, and state transitions
- **Pluggable Planners**: Swap planning strategies (mock, scripted, LLM-based)

### Governed Adaptivity (Horizon 3)
- **Pattern Detection**: Detect behavioral patterns across runs (tool sequences, failures, performance)
- **Suggestion Generation**: Generate policy improvement suggestions from detected patterns
- **Proposal Workflow**: Human-governed policy evolution with approval workflow
- **Policy Versioning**: Immutable version history with rollback capability
- **Visual Inspectors**: Export run data, state machines, and metrics for visualization

## Installation

```bash
go get github.com/felixgeelhaar/agent-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    agent "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func main() {
    // Create engine with tools
    engine, err := agent.NewEngine(
        agent.WithTool(myReadTool),
        agent.WithTool(myWriteTool),
        agent.WithPlanner(myPlanner),
        agent.WithMaxSteps(50),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Run agent with a goal
    run, err := engine.Run(context.Background(), "Process the input file")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Run completed: %s\n", run.Status)
    fmt.Printf("Result: %v\n", run.Result)
}
```

## Architecture

```
agent-go/
├── domain/                 # Core domain layer (no external dependencies)
│   ├── agent/              # Agent aggregate: Run, State, Decision, Evidence
│   ├── tool/               # Tool aggregate: Tool, Annotations, Schema, Registry
│   ├── policy/             # Policy subdomain: Budget, Approval, Constraints, Versioning
│   ├── ledger/             # Audit subdomain: Ledger, Entry, Events
│   ├── artifact/           # Artifact subdomain: Ref, Store
│   ├── pattern/            # Pattern detection: Pattern, Detector, Evidence
│   ├── suggestion/         # Suggestion generation: Suggestion, Generator
│   ├── proposal/           # Policy evolution: Proposal, Status, Changes
│   ├── event/              # Event sourcing: Event, Store
│   ├── run/                # Run management: Run, Store
│   └── inspector/          # Inspection: Inspector, Export formats
│
├── application/            # Application layer (orchestration)
│   ├── engine.go           # Main engine service
│   ├── detection.go        # Pattern detection service
│   ├── evolution.go        # Policy evolution service
│   └── inspection.go       # Inspection service
│
├── infrastructure/         # Infrastructure layer (implementations)
│   ├── statemachine/       # Statekit integration
│   ├── resilience/         # Fortify integration (circuit breaker, retry)
│   ├── logging/            # Bolt integration
│   ├── storage/            # Memory and filesystem stores
│   ├── planner/            # Planner implementations
│   ├── pattern/            # Pattern detectors (sequence, failure, performance)
│   ├── suggestion/         # Suggestion generators (eligibility, budget)
│   ├── proposal/           # Proposal workflow and policy applier
│   ├── inspector/          # Exporters (JSON, DOT, Mermaid, metrics)
│   └── analytics/          # Run analytics and aggregation
│
├── interfaces/             # Interface adapters
│   └── api/                # Public API and builders
│
└── example/                # Example applications
    └── fileops/            # File operations demo
```

## State Machine

The agent operates through a well-defined state machine:

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

### State Semantics

| State | Description | Side Effects Allowed |
|-------|-------------|---------------------|
| `intake` | Initial state, goal parsing | No |
| `explore` | Information gathering | Read-only |
| `decide` | Planning next action | No |
| `act` | Execute tools | Yes |
| `validate` | Verify results | Read-only |
| `done` | Successful completion | No |
| `failed` | Terminal failure | No |

## Tools

Tools are the agent's capabilities. Each tool has:
- **Name**: Unique identifier
- **Schema**: JSON Schema for input/output validation
- **Annotations**: Behavioral metadata
- **Handler**: Execution function

### Creating Tools

```go
import (
    "github.com/felixgeelhaar/agent-go/interfaces/api"
    "github.com/felixgeelhaar/agent-go/domain/tool"
)

readFile := api.NewToolBuilder("read_file").
    WithDescription("Reads content from a file").
    WithAnnotations(api.Annotations{
        ReadOnly:   true,
        Idempotent: true,
        Cacheable:  true,
        RiskLevel:  api.RiskLow,
    }).
    WithInputSchema(tool.NewSchema(json.RawMessage(`{
        "type": "object",
        "properties": {
            "path": {"type": "string"}
        },
        "required": ["path"]
    }`))).
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        var in struct{ Path string `json:"path"` }
        json.Unmarshal(input, &in)

        content, err := os.ReadFile(in.Path)
        if err != nil {
            return tool.Result{}, err
        }

        output, _ := json.Marshal(map[string]any{"content": string(content)})
        return tool.Result{Output: output}, nil
    }).
    MustBuild()
```

### Tool Annotations

| Annotation | Description |
|------------|-------------|
| `ReadOnly` | Tool doesn't modify state |
| `Destructive` | Tool may cause irreversible changes |
| `Idempotent` | Safe to retry on failure |
| `Cacheable` | Results can be cached |
| `RiskLevel` | None, Low, Medium, High, Critical |

## Planners

Planners decide what action the agent should take. Implement the `Planner` interface:

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

### Built-in Planners

- **MockPlanner**: Returns pre-configured decisions (testing)
- **ScriptedPlanner**: Follows a script of decisions (deterministic tests)

## Policy Enforcement

### Budgets

Limit resource consumption:

```go
engine, _ := agent.NewEngine(
    agent.WithBudget("tool_calls", 100),
    agent.WithBudget("tokens", 50000),
)
```

### Approval Workflow

Require human approval for high-risk operations:

```go
engine, _ := agent.NewEngine(
    agent.WithApprover(myApprover),
)

// Tools with RiskHigh or Destructive=true will require approval
```

### Tool Eligibility

Control which tools are available in each state:

```go
engine, _ := agent.NewEngine(
    agent.WithToolEligibility(map[agent.State][]string{
        agent.StateExplore: {"read_file", "list_dir"},
        agent.StateAct:     {"read_file", "write_file", "delete_file"},
    }),
)
```

## Resilience

The runtime uses [fortify](https://github.com/felixgeelhaar/fortify) for resilience:

- **Circuit Breaker**: Prevents cascading failures
- **Retry**: Automatic retry with exponential backoff (idempotent tools only)
- **Bulkhead**: Limits concurrent tool executions
- **Timeout**: Per-execution time limits

```go
engine, _ := agent.NewEngine(
    agent.WithExecutorConfig(resilience.ExecutorConfig{
        MaxConcurrent:           10,
        CircuitBreakerThreshold: 5,
        CircuitBreakerTimeout:   30 * time.Second,
        RetryMaxAttempts:        3,
        RetryInitialDelay:       100 * time.Millisecond,
        DefaultTimeout:          30 * time.Second,
    }),
)
```

## Logging

Structured logging via [bolt](https://github.com/felixgeelhaar/bolt):

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/logging"

// Initialize with options
logging.Init(bolt.WithLevel(bolt.LevelDebug))

// Logs include: run_id, state, tool, decision, duration, etc.
```

## Governed Adaptivity (Horizon 3)

Horizon 3 introduces human-governed learning capabilities. The system detects patterns across runs, generates improvement suggestions, and presents them through a proposal workflow requiring explicit human approval.

**Key Constraint**: No unsupervised self-modification. All policy changes require explicit human approval.

### Pattern Detection

Detect behavioral patterns across runs:

```go
import (
    "github.com/felixgeelhaar/agent-go/domain/pattern"
    infra "github.com/felixgeelhaar/agent-go/infrastructure/pattern"
)

// Create pattern detectors
sequenceDetector := infra.NewSequenceDetector(eventStore)
failureDetector := infra.NewFailureDetector(eventStore)
performanceDetector := infra.NewPerformanceDetector(eventStore)

// Combine into composite detector
detector := infra.NewCompositeDetector(
    sequenceDetector,
    failureDetector,
    performanceDetector,
)

// Detect patterns
patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
    MinConfidence: 0.7,
    MinFrequency:  3,
})
```

#### Pattern Types

| Type | Description |
|------|-------------|
| `tool_sequence` | Repeated sequences of tool calls |
| `recurring_failure` | Same failure modes across runs |
| `tool_failure` | Tools consistently failing |
| `budget_exhaustion` | Runs hitting budget limits |
| `slow_tool` | Tool performance degradation |
| `long_runs` | Runs exceeding expected duration |

### Suggestion Generation

Generate policy improvements from detected patterns:

```go
import (
    "github.com/felixgeelhaar/agent-go/domain/suggestion"
    infra "github.com/felixgeelhaar/agent-go/infrastructure/suggestion"
)

// Create generators
eligibilityGen := infra.NewEligibilityGenerator()
budgetGen := infra.NewBudgetGenerator()

// Combine generators
generator := infra.NewCompositeGenerator(eligibilityGen, budgetGen)

// Generate suggestions from patterns
suggestions, err := generator.Generate(ctx, patterns)

for _, s := range suggestions {
    fmt.Printf("Suggestion: %s\n", s.Title)
    fmt.Printf("  Type: %s\n", s.Type)
    fmt.Printf("  Confidence: %.2f\n", s.Confidence)
    fmt.Printf("  Rationale: %s\n", s.Rationale)
}
```

#### Suggestion Types

| Type | Description |
|------|-------------|
| `add_eligibility` | Allow a tool in a new state |
| `remove_eligibility` | Restrict tool from a state |
| `increase_budget` | Increase budget limit |
| `decrease_budget` | Decrease budget limit |
| `require_approval` | Add approval requirement |

### Proposal Workflow

Convert suggestions into proposals requiring human approval:

```go
import (
    "github.com/felixgeelhaar/agent-go/domain/proposal"
    infra "github.com/felixgeelhaar/agent-go/infrastructure/proposal"
)

// Create workflow service
workflow := infra.NewWorkflowService(
    proposalStore,
    versionStore,
    eventPublisher,
)

// Create proposal from suggestion
prop, err := workflow.CreateFromSuggestion(ctx, suggestion, "system")

// Add custom changes
err = workflow.AddChange(ctx, prop.ID, proposal.PolicyChange{
    Type:   proposal.ChangeTypeAddEligibility,
    Target: "explore",
    Value:  "analyze_file",
})

// Submit for review
err = workflow.Submit(ctx, prop.ID, "developer@example.com")

// Approve (requires human actor)
err = workflow.Approve(ctx, prop.ID, "admin@example.com", "Looks good")

// Apply to policy
err = workflow.Apply(ctx, prop.ID)

// Rollback if needed
err = workflow.Rollback(ctx, prop.ID, "Caused performance issues")
```

#### Proposal Status Flow

```
┌───────┐     ┌────────────────┐     ┌──────────┐     ┌─────────┐
│ draft │────►│ pending_review │────►│ approved │────►│ applied │
└───────┘     └────────────────┘     └──────────┘     └─────────┘
    ▲                │                     │               │
    │                ▼                     ▼               ▼
    │          ┌──────────┐          ┌──────────┐   ┌─────────────┐
    └──────────│ rejected │          │ rejected │   │ rolled_back │
               └──────────┘          └──────────┘   └─────────────┘
```

### Policy Versioning

Every policy change is versioned:

```go
import "github.com/felixgeelhaar/agent-go/domain/policy"

// Get current version
version, err := versionStore.GetCurrent(ctx)
fmt.Printf("Policy Version: %d\n", version.Version)
fmt.Printf("Last Modified: %s\n", version.CreatedAt)

// List version history
versions, err := versionStore.List(ctx)
for _, v := range versions {
    fmt.Printf("v%d - %s (proposal: %s)\n", v.Version, v.CreatedAt, v.ProposalID)
}

// Rollback to previous version
err = workflow.Rollback(ctx, proposalID, "Rolling back to v2")
```

### Visual Inspectors

Export run data for visualization:

```go
import (
    "github.com/felixgeelhaar/agent-go/domain/inspector"
    infra "github.com/felixgeelhaar/agent-go/infrastructure/inspector"
)

// Create exporters
jsonExporter := infra.NewJSONExporter(runStore, eventStore, infra.WithPrettyPrint())
dotExporter := infra.NewDOTExporter(eligibility, transitions)
mermaidExporter := infra.NewMermaidExporter(eligibility, transitions)
metricsExporter := infra.NewMetricsExporter(analytics)

// Create inspector
insp := infra.NewDefaultInspector(jsonExporter, dotExporter, metricsExporter)

// Export run data as JSON
jsonData, err := jsonExporter.ExportRun(ctx, runID)

// Export state machine as DOT (Graphviz)
dotData, err := dotExporter.ExportStateMachine(ctx)

// Export state machine as Mermaid diagram
mermaidData, err := mermaidExporter.ExportStateMachine(ctx)

// Export metrics dashboard data
metricsData, err := metricsExporter.ExportMetrics(ctx, analytics.Filter{})
```

#### Export Formats

| Format | Use Case |
|--------|----------|
| JSON | Run details, events, tool calls for programmatic access |
| DOT | State machine visualization in Graphviz |
| Mermaid | State machine visualization in Markdown |
| Metrics | Dashboard data (success rates, durations, tool usage) |

## Examples

### File Operations

See `example/fileops/` for a complete example demonstrating:
- Tool creation with path traversal protection
- Scripted planner for deterministic execution
- Full agent lifecycle

```bash
go run ./example/fileops
```

## Testing

```bash
# Run all tests
go test ./...

# Run invariant tests
go test -v -run TestInvariant ./test/...

# Security scan
verdict scan

# Coverage
coverctl check
```

## Design Invariants

The runtime enforces these invariants (tested in `test/invariant_test.go` and `test/horizon3_e2e_test.go`):

### Core Runtime Invariants

1. **Tool Eligibility**: Tools execute only in allowed states
2. **Transition Validity**: Only valid state transitions succeed
3. **Approval Enforcement**: Destructive tools require approval
4. **Budget Enforcement**: Execution stops when budget exhausted
5. **Tool Registration**: Duplicate tools rejected
6. **Run Lifecycle**: Proper status transitions
7. **Evidence Accumulation**: Append-only with sequential timestamps
8. **Ledger Immutability**: Audit trail is append-only

### Horizon 3 Invariants

9. **No Unsupervised Modification**: All policy changes require explicit human approval
10. **Audit Trail**: Every proposal action recorded with actor, timestamp, reason
11. **Rollback Capability**: Any applied change can be rolled back
12. **Suggestion-Only Patterns**: Patterns generate suggestions, never direct changes
13. **Version Immutability**: Policy versions are append-only
14. **Human Actor Requirement**: Approve and Apply require non-system actor

## Dependencies

- [statekit](https://github.com/felixgeelhaar/statekit) - Statechart execution engine
- [fortify](https://github.com/felixgeelhaar/fortify) - Resilience patterns
- [bolt](https://github.com/felixgeelhaar/bolt) - Structured logging

## License

MIT License - see [LICENSE](LICENSE) for details.
