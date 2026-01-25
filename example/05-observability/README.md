# 05 - Observability

Demonstrates OpenTelemetry integration for distributed tracing, metrics collection, and structured logging.

## What This Example Shows

- Setting up OpenTelemetry tracer and meter
- Using tracing middleware to capture spans
- Metrics collection for tool executions
- Structured logging with correlation IDs

## Run It

```bash
go run main.go
```

## Expected Output

```
=== Observability Example ===

Tracer and meter initialized

Running agent with tracing enabled...

=== Run Complete ===
Status: done
Steps: 5
Duration: 125ms

=== What Was Captured ===

Traces (viewable in Jaeger/Honeycomb/etc.):
  - agent.run (root span)
    - tool.execute: fast_operation (10ms)
    - tool.execute: slow_operation (100ms)
    - tool.execute: fast_operation (10ms)
```

## Setting Up Tracing

### Basic Setup (stdout)

```go
import "github.com/felixgeelhaar/agent-go/contrib/otel"

tracer, _ := otel.NewTracer("my-agent",
    otel.WithServiceVersion("1.0.0"),
)
defer tracer.Shutdown(ctx)
```

### Production Setup (OTLP)

```go
tracer, _ := otel.NewTracer("my-agent",
    otel.WithOTLPEndpoint("localhost:4317"),
    otel.WithServiceVersion("1.0.0"),
    otel.WithEnvironment("production"),
)
```

### With Jaeger

```bash
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# Run agent
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run main.go

# View traces at http://localhost:16686
```

## Setting Up Metrics

```go
meter, _ := observability.NewMeter("my-agent")

// Metrics are automatically collected:
// - tool_executions_total
// - tool_duration_seconds
// - runs_total
// - budget_remaining
```

## Middleware Configuration

```go
engine, _ := agent.New(
    agent.WithMiddleware(
        // Tracing: Creates spans for each tool execution
        observability.TracingMiddleware(tracer),

        // Metrics: Records counters and histograms
        observability.MetricsMiddleware(meter),

        // Logging: Structured JSON logs
        observability.LoggingMiddleware(),
    ),
)
```

## Trace Attributes

Each span includes:

| Attribute | Description |
|-----------|-------------|
| `agent.run_id` | Unique run identifier |
| `agent.state` | Current state (explore, act, etc.) |
| `tool.name` | Tool being executed |
| `tool.risk_level` | Tool's risk classification |
| `tool.duration_ms` | Execution time |

## Metrics Reference

### Counters

| Metric | Labels | Description |
|--------|--------|-------------|
| `tool_executions_total` | tool, state, status | Number of tool calls |
| `runs_total` | status | Number of agent runs |
| `decisions_total` | type, state | Planner decisions |

### Histograms

| Metric | Labels | Description |
|--------|--------|-------------|
| `tool_duration_seconds` | tool | Tool execution latency |
| `run_duration_seconds` | | Total run duration |
| `planner_latency_seconds` | provider | LLM planning latency |

### Gauges

| Metric | Labels | Description |
|--------|--------|-------------|
| `active_runs` | | Currently running agents |
| `budget_remaining` | name | Remaining budget |

## Structured Logging

Logs include correlation IDs for tracing:

```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:00Z",
  "run_id": "run-abc123",
  "trace_id": "abc123def456",
  "span_id": "789xyz",
  "state": "explore",
  "tool": "read_file",
  "duration_ms": 45,
  "message": "tool execution completed"
}
```

## Next Steps

- **[06-distributed](../06-distributed/)** - Scale with multiple workers
- **[07-production](../07-production/)** - Full production setup
