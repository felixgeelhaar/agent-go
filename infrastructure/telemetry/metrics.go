// Package telemetry provides observability infrastructure including
// OpenTelemetry metrics support for the agent-go runtime.
package telemetry

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsProvider provides access to metrics instruments.
type MetricsProvider struct {
	meter metric.Meter

	// Counters
	toolExecutions    metric.Int64Counter
	stateTransitions  metric.Int64Counter
	budgetConsumption metric.Int64Counter
	rateLimitHits     metric.Int64Counter
	cacheHits         metric.Int64Counter
	cacheMisses       metric.Int64Counter
	errors            metric.Int64Counter

	// Histograms
	toolDuration      metric.Float64Histogram
	planningDuration  metric.Float64Histogram
	runDuration       metric.Float64Histogram

	// Gauges (using UpDownCounter for OpenTelemetry)
	activeRuns        metric.Int64UpDownCounter
	circuitBreakerOpen metric.Int64UpDownCounter

	initOnce sync.Once
	initErr  error
}

// MetricsConfig configures the metrics provider.
type MetricsConfig struct {
	// MeterName is the name of the meter (default: "github.com/felixgeelhaar/agent-go").
	MeterName string
	// MeterVersion is the version of the meter.
	MeterVersion string
	// Attributes are default attributes to attach to all metrics.
	Attributes []attribute.KeyValue
}

// DefaultMetricsConfig returns a default metrics configuration.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		MeterName:    "github.com/felixgeelhaar/agent-go",
		MeterVersion: "1.0.0",
	}
}

// NewMetricsProvider creates a new metrics provider.
func NewMetricsProvider(config MetricsConfig) *MetricsProvider {
	if config.MeterName == "" {
		config = DefaultMetricsConfig()
	}

	provider := otel.GetMeterProvider()
	meter := provider.Meter(
		config.MeterName,
		metric.WithInstrumentationVersion(config.MeterVersion),
	)

	mp := &MetricsProvider{
		meter: meter,
	}

	mp.initOnce.Do(func() {
		mp.initErr = mp.initInstruments()
	})

	return mp
}

// initInstruments initializes all metric instruments.
func (mp *MetricsProvider) initInstruments() error {
	var err error

	// Counters
	mp.toolExecutions, err = mp.meter.Int64Counter(
		"agent.tool.executions",
		metric.WithDescription("Number of tool executions"),
		metric.WithUnit("{execution}"),
	)
	if err != nil {
		return err
	}

	mp.stateTransitions, err = mp.meter.Int64Counter(
		"agent.state.transitions",
		metric.WithDescription("Number of state transitions"),
		metric.WithUnit("{transition}"),
	)
	if err != nil {
		return err
	}

	mp.budgetConsumption, err = mp.meter.Int64Counter(
		"agent.budget.consumption",
		metric.WithDescription("Budget units consumed"),
		metric.WithUnit("{unit}"),
	)
	if err != nil {
		return err
	}

	mp.rateLimitHits, err = mp.meter.Int64Counter(
		"agent.ratelimit.hits",
		metric.WithDescription("Number of rate limit hits"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return err
	}

	mp.cacheHits, err = mp.meter.Int64Counter(
		"agent.cache.hits",
		metric.WithDescription("Number of cache hits"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return err
	}

	mp.cacheMisses, err = mp.meter.Int64Counter(
		"agent.cache.misses",
		metric.WithDescription("Number of cache misses"),
		metric.WithUnit("{miss}"),
	)
	if err != nil {
		return err
	}

	mp.errors, err = mp.meter.Int64Counter(
		"agent.errors",
		metric.WithDescription("Number of errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// Histograms
	mp.toolDuration, err = mp.meter.Float64Histogram(
		"agent.tool.duration",
		metric.WithDescription("Duration of tool executions"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	mp.planningDuration, err = mp.meter.Float64Histogram(
		"agent.planning.duration",
		metric.WithDescription("Duration of planning operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	mp.runDuration, err = mp.meter.Float64Histogram(
		"agent.run.duration",
		metric.WithDescription("Duration of agent runs"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	// Gauges (UpDownCounters)
	mp.activeRuns, err = mp.meter.Int64UpDownCounter(
		"agent.runs.active",
		metric.WithDescription("Number of active agent runs"),
		metric.WithUnit("{run}"),
	)
	if err != nil {
		return err
	}

	mp.circuitBreakerOpen, err = mp.meter.Int64UpDownCounter(
		"agent.circuitbreaker.open",
		metric.WithDescription("Number of open circuit breakers"),
		metric.WithUnit("{circuit}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// Error returns any initialization error.
func (mp *MetricsProvider) Error() error {
	return mp.initErr
}

// RecordToolExecution records a tool execution.
func (mp *MetricsProvider) RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
		attribute.String("agent.state", state),
		attribute.Bool("success", success),
	}

	mp.toolExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	mp.toolDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

	if !success {
		mp.errors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error.type", "tool_execution"),
			attribute.String("tool.name", toolName),
		))
	}
}

// RecordStateTransition records a state transition.
func (mp *MetricsProvider) RecordStateTransition(ctx context.Context, fromState, toState string, runID string) {
	attrs := []attribute.KeyValue{
		attribute.String("state.from", fromState),
		attribute.String("state.to", toState),
		attribute.String("run.id", runID),
	}

	mp.stateTransitions.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordBudgetConsumption records budget consumption.
func (mp *MetricsProvider) RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64) {
	attrs := []attribute.KeyValue{
		attribute.String("budget.name", budgetName),
		attribute.Int64("budget.remaining", remaining),
	}

	mp.budgetConsumption.Add(ctx, amount, metric.WithAttributes(attrs...))
}

// RecordRateLimitHit records a rate limit hit.
func (mp *MetricsProvider) RecordRateLimitHit(ctx context.Context, toolName string) {
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}

	mp.rateLimitHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordCacheHit records a cache hit.
func (mp *MetricsProvider) RecordCacheHit(ctx context.Context, toolName string) {
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}

	mp.cacheHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordCacheMiss records a cache miss.
func (mp *MetricsProvider) RecordCacheMiss(ctx context.Context, toolName string) {
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}

	mp.cacheMisses.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordError records an error.
func (mp *MetricsProvider) RecordError(ctx context.Context, errorType string, details map[string]string) {
	attrs := []attribute.KeyValue{
		attribute.String("error.type", errorType),
	}
	for k, v := range details {
		attrs = append(attrs, attribute.String(k, v))
	}

	mp.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordPlanningDuration records the duration of a planning operation.
func (mp *MetricsProvider) RecordPlanningDuration(ctx context.Context, duration time.Duration, state string) {
	attrs := []attribute.KeyValue{
		attribute.String("agent.state", state),
	}

	mp.planningDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// RecordRunDuration records the duration of an agent run.
func (mp *MetricsProvider) RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("state.final", finalState),
		attribute.Bool("success", success),
	}

	mp.runDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// IncrementActiveRuns increments the active runs counter.
func (mp *MetricsProvider) IncrementActiveRuns(ctx context.Context) {
	mp.activeRuns.Add(ctx, 1)
}

// DecrementActiveRuns decrements the active runs counter.
func (mp *MetricsProvider) DecrementActiveRuns(ctx context.Context) {
	mp.activeRuns.Add(ctx, -1)
}

// RecordCircuitBreakerStateChange records a circuit breaker state change.
func (mp *MetricsProvider) RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool) {
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}

	if isOpen {
		mp.circuitBreakerOpen.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		mp.circuitBreakerOpen.Add(ctx, -1, metric.WithAttributes(attrs...))
	}
}

// NoopMetricsProvider is a no-op metrics provider for testing or when metrics are disabled.
type NoopMetricsProvider struct{}

// RecordToolExecution is a no-op.
func (n *NoopMetricsProvider) RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration) {}

// RecordStateTransition is a no-op.
func (n *NoopMetricsProvider) RecordStateTransition(ctx context.Context, fromState, toState string, runID string) {}

// RecordBudgetConsumption is a no-op.
func (n *NoopMetricsProvider) RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64) {}

// RecordRateLimitHit is a no-op.
func (n *NoopMetricsProvider) RecordRateLimitHit(ctx context.Context, toolName string) {}

// RecordCacheHit is a no-op.
func (n *NoopMetricsProvider) RecordCacheHit(ctx context.Context, toolName string) {}

// RecordCacheMiss is a no-op.
func (n *NoopMetricsProvider) RecordCacheMiss(ctx context.Context, toolName string) {}

// RecordError is a no-op.
func (n *NoopMetricsProvider) RecordError(ctx context.Context, errorType string, details map[string]string) {}

// RecordPlanningDuration is a no-op.
func (n *NoopMetricsProvider) RecordPlanningDuration(ctx context.Context, duration time.Duration, state string) {}

// RecordRunDuration is a no-op.
func (n *NoopMetricsProvider) RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool) {}

// IncrementActiveRuns is a no-op.
func (n *NoopMetricsProvider) IncrementActiveRuns(ctx context.Context) {}

// DecrementActiveRuns is a no-op.
func (n *NoopMetricsProvider) DecrementActiveRuns(ctx context.Context) {}

// RecordCircuitBreakerStateChange is a no-op.
func (n *NoopMetricsProvider) RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool) {}

// Metrics defines the interface for metrics recording.
type Metrics interface {
	RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration)
	RecordStateTransition(ctx context.Context, fromState, toState string, runID string)
	RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64)
	RecordRateLimitHit(ctx context.Context, toolName string)
	RecordCacheHit(ctx context.Context, toolName string)
	RecordCacheMiss(ctx context.Context, toolName string)
	RecordError(ctx context.Context, errorType string, details map[string]string)
	RecordPlanningDuration(ctx context.Context, duration time.Duration, state string)
	RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool)
	IncrementActiveRuns(ctx context.Context)
	DecrementActiveRuns(ctx context.Context)
	RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool)
}

// Ensure implementations satisfy the interface.
var (
	_ Metrics = (*MetricsProvider)(nil)
	_ Metrics = (*NoopMetricsProvider)(nil)
)
