package observability

import (
	"context"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/telemetry"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// TracingMiddleware creates middleware that traces tool executions.
func TracingMiddleware(tracer telemetry.Tracer) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			ctx, span := tracer.StartSpan(ctx, "tool.execute",
				telemetry.WithAttributes(
					telemetry.String("tool.name", execCtx.Tool.Name()),
					telemetry.String("agent.state", string(execCtx.CurrentState)),
					telemetry.String("agent.run_id", execCtx.RunID),
					telemetry.String("tool.reason", execCtx.Reason),
				),
				telemetry.WithSpanKind(telemetry.SpanKindInternal),
			)
			defer span.End()

			// Add tool annotations as attributes
			annotations := execCtx.Tool.Annotations()
			span.SetAttributes(
				telemetry.Bool("tool.read_only", annotations.ReadOnly),
				telemetry.Bool("tool.destructive", annotations.Destructive),
				telemetry.Bool("tool.idempotent", annotations.Idempotent),
			)

			// Execute the tool
			result, err := next(ctx, execCtx)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(telemetry.StatusCodeError, err.Error())
				span.SetAttributes(telemetry.String("tool.status", "error"))
			} else {
				span.SetStatus(telemetry.StatusCodeOK, "")
				span.SetAttributes(telemetry.String("tool.status", "success"))
			}

			return result, err
		}
	}
}

// MetricsMiddleware creates middleware that records tool execution metrics.
func MetricsMiddleware(meter telemetry.Meter) middleware.Middleware {
	// Create metrics instruments
	executionCounter := meter.Counter("agent.tool.executions_total",
		telemetry.WithDescription("Total number of tool executions"),
		telemetry.WithUnit("{execution}"),
	)

	executionDuration := meter.Histogram("agent.tool.duration_seconds",
		telemetry.WithDescription("Duration of tool executions"),
		telemetry.WithUnit("s"),
	)

	errorCounter := meter.Counter("agent.tool.errors_total",
		telemetry.WithDescription("Total number of tool execution errors"),
		telemetry.WithUnit("{error}"),
	)

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			// Execute the tool
			result, err := next(ctx, execCtx)

			// Record metrics
			duration := time.Since(start).Seconds()
			attrs := []telemetry.Attribute{
				telemetry.String("tool", execCtx.Tool.Name()),
				telemetry.String("state", string(execCtx.CurrentState)),
			}

			status := "success"
			if err != nil {
				status = "error"
				errorCounter.Add(ctx, 1, attrs...)
			}

			attrs = append(attrs, telemetry.String("status", status))
			executionCounter.Add(ctx, 1, attrs...)
			executionDuration.Record(ctx, duration, attrs...)

			return result, err
		}
	}
}

// CombinedMiddleware creates middleware that combines tracing and metrics.
func CombinedMiddleware(tracer telemetry.Tracer, meter telemetry.Meter) middleware.Middleware {
	tracingMw := TracingMiddleware(tracer)
	metricsMw := MetricsMiddleware(meter)

	return func(next middleware.Handler) middleware.Handler {
		// Chain: tracing wraps metrics wraps handler
		return tracingMw(metricsMw(next))
	}
}

// AgentMetrics provides pre-built metrics for agent operations.
type AgentMetrics struct {
	// RunsTotal counts total agent runs.
	RunsTotal telemetry.Counter

	// RunDuration records run duration.
	RunDuration telemetry.Histogram

	// DecisionsTotal counts planner decisions.
	DecisionsTotal telemetry.Counter

	// PlannerLatency records planner call latency.
	PlannerLatency telemetry.Histogram

	// ActiveRuns tracks concurrent runs.
	ActiveRuns telemetry.Gauge

	// BudgetRemaining tracks remaining budget.
	BudgetRemaining telemetry.Gauge
}

// NewAgentMetrics creates agent metrics.
func NewAgentMetrics(meter telemetry.Meter) *AgentMetrics {
	return &AgentMetrics{
		RunsTotal: meter.Counter("agent.runs_total",
			telemetry.WithDescription("Total number of agent runs"),
			telemetry.WithUnit("{run}"),
		),
		RunDuration: meter.Histogram("agent.run.duration_seconds",
			telemetry.WithDescription("Duration of agent runs"),
			telemetry.WithUnit("s"),
		),
		DecisionsTotal: meter.Counter("agent.decisions_total",
			telemetry.WithDescription("Total number of planner decisions"),
			telemetry.WithUnit("{decision}"),
		),
		PlannerLatency: meter.Histogram("agent.planner.latency_seconds",
			telemetry.WithDescription("Latency of planner calls"),
			telemetry.WithUnit("s"),
		),
		ActiveRuns: meter.Gauge("agent.runs_active",
			telemetry.WithDescription("Number of active agent runs"),
			telemetry.WithUnit("{run}"),
		),
		BudgetRemaining: meter.Gauge("agent.budget_remaining",
			telemetry.WithDescription("Remaining budget"),
			telemetry.WithUnit("{unit}"),
		),
	}
}

// RecordRunStart records the start of a run.
func (m *AgentMetrics) RecordRunStart(ctx context.Context, runID string) {
	m.ActiveRuns.Record(ctx, 1, telemetry.String("run_id", runID))
}

// RecordRunEnd records the end of a run.
func (m *AgentMetrics) RecordRunEnd(ctx context.Context, status string, duration time.Duration) {
	m.RunsTotal.Add(ctx, 1, telemetry.String("status", status))
	m.RunDuration.Record(ctx, duration.Seconds(), telemetry.String("status", status))
}

// RecordDecision records a planner decision.
func (m *AgentMetrics) RecordDecision(ctx context.Context, decisionType, state string, latency time.Duration) {
	attrs := []telemetry.Attribute{
		telemetry.String("type", decisionType),
		telemetry.String("state", state),
	}
	m.DecisionsTotal.Add(ctx, 1, attrs...)
	m.PlannerLatency.Record(ctx, latency.Seconds(), attrs...)
}

// RecordBudget records current budget state.
func (m *AgentMetrics) RecordBudget(ctx context.Context, name string, remaining int) {
	m.BudgetRemaining.Record(ctx, float64(remaining), telemetry.String("budget", name))
}
