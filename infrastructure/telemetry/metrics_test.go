package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// setupTestMetrics sets up a test meter provider and returns it along with a reader.
func setupTestMetrics(t *testing.T) (*metric.ManualReader, *MetricsProvider) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

	mp := NewMetricsProvider(DefaultMetricsConfig())
	if mp.Error() != nil {
		t.Fatalf("failed to create metrics provider: %v", mp.Error())
	}

	return reader, mp
}

func TestNewMetricsProvider(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	if mp == nil {
		t.Fatal("NewMetricsProvider returned nil")
	}
	if mp.Error() != nil {
		t.Errorf("unexpected error: %v", mp.Error())
	}
}

func TestMetricsProvider_RecordToolExecution(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	// Record a successful execution
	mp.RecordToolExecution(ctx, "test_tool", "explore", true, 100*time.Millisecond)

	// Record a failed execution
	mp.RecordToolExecution(ctx, "test_tool", "act", false, 50*time.Millisecond)

	// Collect and verify metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	// Verify we have metrics
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.tool.executions" {
				found = true
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("expected Sum[int64], got %T", m.Data)
					continue
				}
				// We recorded 2 executions
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				if total != 2 {
					t.Errorf("expected 2 executions, got %d", total)
				}
			}
		}
	}
	if !found {
		t.Error("agent.tool.executions metric not found")
	}
}

func TestMetricsProvider_RecordStateTransition(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordStateTransition(ctx, "intake", "explore", "run-123")
	mp.RecordStateTransition(ctx, "explore", "decide", "run-123")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.state.transitions" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.state.transitions metric not found")
	}
}

func TestMetricsProvider_RecordBudgetConsumption(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordBudgetConsumption(ctx, "tool_calls", 1, 99)
	mp.RecordBudgetConsumption(ctx, "tool_calls", 1, 98)
	mp.RecordBudgetConsumption(ctx, "tokens", 100, 9900)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.budget.consumption" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.budget.consumption metric not found")
	}
}

func TestMetricsProvider_RecordRateLimitHit(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordRateLimitHit(ctx, "expensive_tool")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.ratelimit.hits" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.ratelimit.hits metric not found")
	}
}

func TestMetricsProvider_RecordCacheHitMiss(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordCacheHit(ctx, "cached_tool")
	mp.RecordCacheMiss(ctx, "uncached_tool")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	foundHits := false
	foundMisses := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.cache.hits" {
				foundHits = true
			}
			if m.Name == "agent.cache.misses" {
				foundMisses = true
			}
		}
	}
	if !foundHits {
		t.Error("agent.cache.hits metric not found")
	}
	if !foundMisses {
		t.Error("agent.cache.misses metric not found")
	}
}

func TestMetricsProvider_RecordActiveRuns(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.IncrementActiveRuns(ctx)
	mp.IncrementActiveRuns(ctx)
	mp.DecrementActiveRuns(ctx)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.runs.active" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.runs.active metric not found")
	}
}

func TestMetricsProvider_RecordCircuitBreakerStateChange(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordCircuitBreakerStateChange(ctx, "flaky_tool", true)
	mp.RecordCircuitBreakerStateChange(ctx, "flaky_tool", false)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.circuitbreaker.open" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.circuitbreaker.open metric not found")
	}
}

func TestMetricsProvider_RecordError(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordError(ctx, "validation", map[string]string{
		"tool.name": "test_tool",
		"reason":    "invalid input",
	})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.errors" {
				found = true
			}
		}
	}
	if !found {
		t.Error("agent.errors metric not found")
	}
}

func TestMetricsProvider_RecordDurations(t *testing.T) {
	reader, mp := setupTestMetrics(t)
	defer reader.Shutdown(context.Background())

	ctx := context.Background()

	mp.RecordPlanningDuration(ctx, 50*time.Millisecond, "explore")
	mp.RecordRunDuration(ctx, 1*time.Second, "done", true)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	foundPlanning := false
	foundRun := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agent.planning.duration" {
				foundPlanning = true
			}
			if m.Name == "agent.run.duration" {
				foundRun = true
			}
		}
	}
	if !foundPlanning {
		t.Error("agent.planning.duration metric not found")
	}
	if !foundRun {
		t.Error("agent.run.duration metric not found")
	}
}

func TestNoopMetricsProvider(t *testing.T) {
	// Verify that NoopMetricsProvider doesn't panic
	noop := &NoopMetricsProvider{}
	ctx := context.Background()

	noop.RecordToolExecution(ctx, "tool", "state", true, time.Second)
	noop.RecordStateTransition(ctx, "from", "to", "run")
	noop.RecordBudgetConsumption(ctx, "budget", 1, 99)
	noop.RecordRateLimitHit(ctx, "tool")
	noop.RecordCacheHit(ctx, "tool")
	noop.RecordCacheMiss(ctx, "tool")
	noop.RecordError(ctx, "type", nil)
	noop.RecordPlanningDuration(ctx, time.Second, "state")
	noop.RecordRunDuration(ctx, time.Second, "done", true)
	noop.IncrementActiveRuns(ctx)
	noop.DecrementActiveRuns(ctx)
	noop.RecordCircuitBreakerStateChange(ctx, "tool", true)
}

func TestDefaultMetricsConfig(t *testing.T) {
	config := DefaultMetricsConfig()

	if config.MeterName == "" {
		t.Error("MeterName should not be empty")
	}
	if config.MeterVersion == "" {
		t.Error("MeterVersion should not be empty")
	}
}
