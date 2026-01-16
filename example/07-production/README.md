# 07 - Production

A production-ready agent setup combining all features: LLM planning, observability, security validation, audit logging, and graceful shutdown.

## What This Example Shows

- Configuration from environment variables
- Full observability setup (tracing + metrics)
- Input validation and security
- Audit logging for compliance
- Graceful shutdown handling
- Production error handling patterns

## Run It

```bash
# Basic run (uses scripted planner)
go run main.go

# With LLM (requires API key)
ANTHROPIC_API_KEY=your-key go run main.go

# Full production configuration
ENVIRONMENT=production \
SERVICE_NAME=file-agent \
SERVICE_VERSION=1.2.3 \
ANTHROPIC_API_KEY=your-key \
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 \
go run main.go
```

## Expected Output

```
=== Production Agent Example ===

Environment: development
Service: file-agent v1.0.0

Observability initialized
Security initialized
Tools registered
LLM planner initialized (Anthropic)
Policies configured
Engine created

Running agent...

=== Run Complete ===
Run ID: run-abc123
Status: done
Steps: 3
Duration: 1.234s
Result: {"files": [...]}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Deployment environment | `development` |
| `SERVICE_NAME` | Service name for telemetry | `file-agent` |
| `SERVICE_VERSION` | Service version | `1.0.0` |
| `ANTHROPIC_API_KEY` | Anthropic API key | (none) |
| `OPENAI_API_KEY` | OpenAI API key | (none) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry endpoint | (none) |

## Production Checklist

### Observability

- [ ] Configure OTLP endpoint for traces
- [ ] Set up metrics collection (Prometheus)
- [ ] Configure log aggregation (ELK, Datadog)
- [ ] Set up alerting for error rates

### Security

- [ ] Enable input validation on all tools
- [ ] Configure approval workflow for destructive tools
- [ ] Set up audit logging
- [ ] Review tool risk levels

### Resilience

- [ ] Configure appropriate budgets
- [ ] Set up circuit breakers
- [ ] Configure retry policies
- [ ] Test graceful shutdown

### Deployment

- [ ] Use Redis/NATS for distributed queues
- [ ] Use Redis for distributed locks
- [ ] Configure health checks
- [ ] Set resource limits

## Docker Deployment

### Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o agent ./example/07-production

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/agent .
EXPOSE 8080
CMD ["./agent"]
```

### docker-compose.yml

```yaml
version: '3.8'
services:
  agent:
    build: .
    environment:
      - ENVIRONMENT=production
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317
    depends_on:
      - jaeger
      - redis

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "4317:4317"

  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
```

## Health Checks

```go
// Add health endpoint
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "version": config.ServiceVersion,
    })
})

http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
    // Check dependencies
    if err := checkDependencies(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
})
```

## Graceful Shutdown

The example handles SIGINT and SIGTERM:

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    fmt.Println("Shutting down...")
    cancel() // Cancel context to stop agent
}()
```

## Monitoring

### Key Metrics to Watch

- `tool_executions_total` - Track tool usage
- `tool_duration_seconds` - Identify slow tools
- `runs_total{status=failed}` - Monitor failure rate
- `budget_remaining` - Watch for budget exhaustion

### Alerts to Configure

1. **High Error Rate**: `rate(runs_total{status=failed}[5m]) > 0.1`
2. **Slow Tools**: `histogram_quantile(0.99, tool_duration_seconds) > 10`
3. **Budget Exhaustion**: `budget_remaining < 10`

## Summary

This example demonstrates production patterns for agent-go:

1. **Configuration**: Environment-based config
2. **Observability**: Tracing, metrics, logging
3. **Security**: Validation, audit, approvals
4. **Resilience**: Budgets, timeouts, graceful shutdown
5. **Deployment**: Docker, health checks, monitoring

Use this as a template for production deployments.
