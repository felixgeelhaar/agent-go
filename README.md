# agent-go

[![Go Reference](https://pkg.go.dev/badge/github.com/felixgeelhaar/agent-go.svg)](https://pkg.go.dev/github.com/felixgeelhaar/agent-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/agent-go)](https://goreportcard.com/report/github.com/felixgeelhaar/agent-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Build trustworthy AI agents in Go.** A state-driven runtime where intelligence is constrained by design, not hope.

```go
engine, _ := agent.New(
    agent.WithTools(readFile, writeFile),
    agent.WithPlanner(llmPlanner),
    agent.WithBudget("tool_calls", 50),
)

run, _ := engine.Run(ctx, "Summarize all markdown files in ./docs")
```

---

## Why agent-go?

Most agent frameworks treat safety as an afterthought. agent-go makes **trust the product**:

| Problem | agent-go Solution |
|---------|-------------------|
| Agents run arbitrary code | **State machine** constrains what tools run when |
| No visibility into decisions | **Audit ledger** records every action |
| Runaway costs | **Budget enforcement** with hard limits |
| Dangerous operations | **Approval workflows** for destructive tools |
| Untestable LLM behavior | **Deterministic mode** with scripted planners |
| Python's GIL limits scale | **Native Go concurrency** for high throughput |

**The key insight**: Structure agent behavior through constraints, not prompts.

---

## Quick Start

### Installation

```bash
go get github.com/felixgeelhaar/agent-go
```

### Your First Agent (5 minutes)

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    agent "github.com/felixgeelhaar/agent-go/interfaces/api"
    "github.com/felixgeelhaar/agent-go/domain/tool"
)

func main() {
    // 1. Create a simple tool
    greetTool := agent.NewToolBuilder("greet").
        WithDescription("Greets a person by name").
        WithAnnotations(agent.Annotations{ReadOnly: true}).
        WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
            var in struct{ Name string `json:"name"` }
            json.Unmarshal(input, &in)
            output, _ := json.Marshal(map[string]string{
                "message": fmt.Sprintf("Hello, %s!", in.Name),
            })
            return tool.Result{Output: output}, nil
        }).
        MustBuild()

    // 2. Create a scripted planner (for testing - swap with LLM in production)
    planner := agent.NewScriptedPlanner(
        agent.ScriptStep{
            ExpectState: agent.StateIntake,
            Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
        },
        agent.ScriptStep{
            ExpectState: agent.StateExplore,
            Decision:    agent.NewCallToolDecision("greet", json.RawMessage(`{"name":"World"}`), "greeting user"),
        },
        agent.ScriptStep{
            ExpectState: agent.StateExplore,
            Decision:    agent.NewFinishDecision("completed", json.RawMessage(`{"status":"done"}`)),
        },
    )

    // 3. Build and run the engine
    engine, err := agent.New(
        agent.WithTool(greetTool),
        agent.WithPlanner(planner),
        agent.WithMaxSteps(10),
    )
    if err != nil {
        log.Fatal(err)
    }

    run, err := engine.Run(context.Background(), "Say hello")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Status: %s\n", run.Status)
    fmt.Printf("Result: %s\n", run.Result)
}
```

Run it:
```bash
go run main.go
# Output:
# Status: done
# Result: {"status":"done"}
```

---

## Core Concepts

### State Machine

Agents operate within a **canonical 7-state lifecycle**. Each state has explicit semantics:

```
intake → explore → decide → act → validate → done
                     ↓                  ↓
                  failed ←──────────────┘
```

| State | Purpose | Side Effects |
|-------|---------|--------------|
| `intake` | Parse and normalize the goal | None |
| `explore` | Gather information (read-only tools) | None |
| `decide` | Choose next action | None |
| `act` | Execute side-effecting tools | **Yes** |
| `validate` | Verify results | None |
| `done` | Success (terminal) | None |
| `failed` | Failure (terminal) | None |

**Why this matters**: A tool marked `Destructive` can only run in `act` state. Period.

### Tools

Tools are the agent's capabilities. Each tool declares its behavior through **annotations**:

```go
writeTool := agent.NewToolBuilder("write_file").
    WithAnnotations(agent.Annotations{
        ReadOnly:    false,      // Modifies state
        Destructive: true,       // May cause irreversible changes
        Idempotent:  false,      // Not safe to retry
        RiskLevel:   agent.RiskHigh,
    }).
    WithHandler(writeHandler).
    MustBuild()
```

Annotations drive runtime behavior:
- `ReadOnly` tools can run in `explore` and `validate`
- `Destructive` tools require approval (if configured)
- `Idempotent` tools get automatic retry on failure
- `Cacheable` results are memoized

### Planners

Planners decide what the agent does next. **Swap implementations without changing agent code**:

```go
// Testing: deterministic, no LLM needed
planner := agent.NewScriptedPlanner(steps...)

// Development: local models via Ollama
planner, _ := ollama.New(ollama.WithModel("llama3"))

// Production: Claude, GPT-4, Gemini
planner, _ := anthropic.New(anthropic.WithModel("claude-sonnet-4-20250514"))
```

### Policies

Hard limits that **cannot be overridden by the LLM**:

```go
engine, _ := agent.New(
    // Budget: Stop after 100 tool calls, no matter what
    agent.WithBudget("tool_calls", 100),

    // Approval: Human must approve destructive operations
    agent.WithApprover(myApprover),

    // Eligibility: read_file only in explore, write_file only in act
    agent.WithToolEligibility(eligibility),
)
```

---

## Features

### LLM Providers

Pluggable providers for all major LLMs:

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/planner/provider/anthropic"
import "github.com/felixgeelhaar/agent-go/infrastructure/planner/provider/openai"
import "github.com/felixgeelhaar/agent-go/infrastructure/planner/provider/gemini"
import "github.com/felixgeelhaar/agent-go/infrastructure/planner/provider/ollama"

// Each provider implements the same interface
provider, _ := anthropic.New(anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))
provider, _ := openai.New(openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
provider, _ := gemini.New(gemini.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
provider, _ := ollama.New(ollama.WithBaseURL("http://localhost:11434"))
```

### Domain Packs

Pre-built tool collections for common domains:

```go
import "github.com/felixgeelhaar/agent-go/pack/database"
import "github.com/felixgeelhaar/agent-go/pack/git"
import "github.com/felixgeelhaar/agent-go/pack/kubernetes"
import "github.com/felixgeelhaar/agent-go/pack/cloud"

// Database: query, execute, schema inspection
dbPack := database.New(db, database.WithMaxRows(1000))

// Git: status, log, diff, commit
gitPack := git.New("/path/to/repo", git.WithAllowCommit(true))

// Kubernetes: get, list, logs, apply
k8sPack := kubernetes.New(client, kubernetes.WithNamespace("production"))

// Cloud: S3/GCS/Azure blob operations
cloudPack := cloud.New(provider, cloud.WithBucket("my-bucket"))
```

### Observability

OpenTelemetry integration for traces and metrics:

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/observability"

tracer, _ := observability.NewTracer("my-agent",
    observability.WithOTLPEndpoint("localhost:4317"),
)

engine, _ := agent.New(
    agent.WithMiddleware(observability.TracingMiddleware(tracer)),
)

// Automatic spans for: tool execution, state transitions, planner calls
// Automatic metrics for: tool_duration, run_duration, budget_usage
```

### Security

Input validation, secret management, and audit logging:

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/security/validation"
import "github.com/felixgeelhaar/agent-go/infrastructure/security/secrets"
import "github.com/felixgeelhaar/agent-go/infrastructure/security/audit"

// Validate tool inputs
validator := validation.NewValidator(
    validation.WithRule("path", validation.NoPathTraversal()),
    validation.WithRule("query", validation.NoSQLInjection()),
)

// Manage secrets
secretMgr := secrets.NewEnvManager(secrets.WithPrefix("AGENT_"))

// Audit all operations
auditor := audit.NewLogger(audit.WithOutput(auditFile))
```

### Distributed Execution

Scale across multiple workers:

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/distributed"
import "github.com/felixgeelhaar/agent-go/infrastructure/distributed/queue"
import "github.com/felixgeelhaar/agent-go/infrastructure/distributed/lock"

// Create queue (memory for dev, Redis/NATS for prod)
q := queue.NewMemoryQueue()

// Create distributed lock
l := lock.NewMemoryLock()

// Start workers
worker := distributed.NewWorker(distributed.WorkerConfig{
    Queue:       q,
    Lock:        l,
    Concurrency: 4,
})
worker.Start(ctx)
```

---

## Architecture

```
agent-go/
├── domain/                    # Core domain (no external deps)
│   ├── agent/                 # Run, State, Decision, Evidence
│   ├── tool/                  # Tool, Annotations, Schema, Registry
│   ├── policy/                # Budget, Approval, Eligibility
│   └── ledger/                # Audit trail
│
├── application/               # Orchestration
│   └── engine.go              # Main engine service
│
├── infrastructure/            # Implementations
│   ├── planner/provider/      # LLM providers
│   ├── observability/         # OpenTelemetry
│   ├── security/              # Validation, secrets, audit
│   ├── distributed/           # Queues, locks, workers
│   └── resilience/            # Circuit breaker, retry
│
├── interfaces/api/            # Public API
│
├── pack/                      # Domain tool packs
│   ├── database/
│   ├── git/
│   ├── kubernetes/
│   └── cloud/
│
└── example/                   # Examples
```

---

## Comparison

| Feature | agent-go | LangChain | AutoGPT | CrewAI |
|---------|----------|-----------|---------|--------|
| Language | Go | Python | Python | Python |
| Type Safety | Compile-time | Runtime | Runtime | Runtime |
| State Machine | Built-in | Manual | None | None |
| Policy Enforcement | First-class | Partial | None | Partial |
| Audit Trail | Built-in | Manual | None | None |
| Deterministic Testing | ScriptedPlanner | Difficult | Difficult | Difficult |
| Concurrency | Native goroutines | GIL-limited | Limited | Limited |
| Memory Footprint | ~10MB | ~100MB+ | ~200MB+ | ~150MB+ |

---

## Documentation

- **[Quick Start Guide](docs/quickstart.md)** - Your first agent in 5 minutes
- **[Concepts](docs/concepts/)** - States, tools, planners, policies
- **[Architecture](docs/architecture/)** - DDD structure, layer responsibilities
- **[Integration Guides](docs/integrations/)** - LLM providers, packs, security
- **[Examples](example/)** - Progressive examples from minimal to production
- **[API Reference](https://pkg.go.dev/github.com/felixgeelhaar/agent-go)** - GoDoc

---

## Examples

| Example | Description | Complexity |
|---------|-------------|------------|
| [01-minimal](example/01-minimal/) | Absolute minimum working agent | Beginner |
| [02-tools](example/02-tools/) | Custom tool creation | Beginner |
| [03-policies](example/03-policies/) | Budgets and approvals | Intermediate |
| [04-llm-planner](example/04-llm-planner/) | Real LLM integration | Intermediate |
| [05-observability](example/05-observability/) | Tracing and metrics | Intermediate |
| [06-distributed](example/06-distributed/) | Multi-worker setup | Advanced |
| [07-production](example/07-production/) | Full production setup | Advanced |

---

## Development

```bash
# Build
go build ./...

# Test with race detection
go test -race ./...

# Coverage
coverctl check --fail-under=80

# Security scan
verdict scan

# Lint
golangci-lint run ./...
```

---

## Dependencies

- **[statekit](https://github.com/felixgeelhaar/statekit)** - Statechart execution engine
- **[fortify](https://github.com/felixgeelhaar/fortify)** - Resilience patterns (circuit breaker, retry)
- **[bolt](https://github.com/felixgeelhaar/bolt)** - High-performance structured logging

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

MIT License - see [LICENSE](LICENSE) for details.
