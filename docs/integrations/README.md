# Integration Guides

These guides cover integrating agent-go with external services and infrastructure.

## Contents

| Guide | Description |
|-------|-------------|
| [LLM Providers](./providers/) | Anthropic, OpenAI, Gemini, Ollama integration |
| [Domain Packs](./packs/) | Database, Git, Kubernetes, Cloud tool packs |
| [Security](./security/) | Input validation, secrets, audit logging |
| [Observability](./observability/) | OpenTelemetry tracing and metrics |
| [Distributed](./distributed/) | Multi-worker execution with queues and locks |

## Quick Reference

### LLM Providers

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/planner"

// Anthropic
provider := planner.NewAnthropicProvider(planner.AnthropicConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-sonnet-4-20250514",
})

// OpenAI
provider := planner.NewOpenAIProvider(planner.OpenAIConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4-turbo",
})

// Ollama (local)
provider := planner.NewOllamaProvider(planner.OllamaConfig{
    BaseURL: "http://localhost:11434",
    Model:   "llama3",
})

// Use with LLM planner
llmPlanner := planner.NewLLMPlanner(planner.LLMPlannerConfig{
    Provider: provider,
})
```

### Domain Packs

```go
import (
    "github.com/felixgeelhaar/agent-go/pack/database"
    "github.com/felixgeelhaar/agent-go/pack/git"
)

// Database pack
dbPack, _ := database.New(db, database.WithWriteAccess())

// Git pack
gitPack, _ := git.New("/path/to/repo")

// Use with engine
engine, _ := api.New(
    api.WithPack(dbPack),
    api.WithPack(gitPack),
)
```

### Security

```go
import (
    "github.com/felixgeelhaar/agent-go/infrastructure/security/validation"
    "github.com/felixgeelhaar/agent-go/infrastructure/security/audit"
)

// Validation
schema := validation.NewSchema().
    AddRule("path", validation.Required()).
    AddRule("path", validation.NoPathTraversal())

// Audit logging
auditLogger := audit.NewJSONLogger(os.Stdout)

// Use with engine
engine, _ := api.New(
    api.WithMiddleware(
        validation.ValidationMiddleware(schemas),
        audit.AuditMiddleware(auditLogger),
    ),
)
```

### Observability

```go
import "github.com/felixgeelhaar/agent-go/infrastructure/observability"

tracer := observability.NewOTelTracer("my-service")
meter := observability.NewOTelMeter("my-service")

engine, _ := api.New(
    api.WithMiddleware(
        observability.TracingMiddleware(tracer),
        observability.MetricsMiddleware(meter),
    ),
)
```

### Distributed Execution

```go
import (
    "github.com/felixgeelhaar/agent-go/infrastructure/distributed"
    "github.com/felixgeelhaar/agent-go/infrastructure/distributed/queue"
    "github.com/felixgeelhaar/agent-go/infrastructure/distributed/lock"
)

taskQueue := queue.NewMemoryQueue() // Use Redis in production
distLock := lock.NewMemoryLock()

worker := distributed.NewWorker(distributed.WorkerConfig{
    ID:       "worker-1",
    Queue:    taskQueue,
    Lock:     distLock,
    Registry: registry,
},
    distributed.WithConcurrency(4),
)

go worker.Start(ctx)
```

## See Also

- [Quick Start](../quickstart.md) - Get started quickly
- [Concepts](../concepts/) - Core concepts explained
- [Examples](../../example/) - Working code examples
