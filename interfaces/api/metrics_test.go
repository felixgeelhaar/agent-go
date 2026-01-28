package api_test

import (
	"testing"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func TestDefaultMetricsConfig(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultMetricsConfig()
	_ = cfg
}

func TestNewMetricsProvider(t *testing.T) {
	t.Parallel()
	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
	if provider == nil {
		t.Fatal("NewMetricsProvider() returned nil")
	}
}

func TestWithMetrics(t *testing.T) {
	t.Parallel()

	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)

	engine, err := api.New(
		api.WithPlanner(mockPlanner),
		api.WithMetrics(provider),
	)
	if err != nil {
		t.Fatalf("New() with WithMetrics error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() with WithMetrics returned nil engine")
	}
}

func TestNewCacheMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
	recorder := api.NewCacheMetricsRecorder(provider)
	_ = recorder
}

func TestNewRateLimitMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
	recorder := api.NewRateLimitMetricsRecorder(provider)
	_ = recorder
}

func TestNewCircuitBreakerMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
	recorder := api.NewCircuitBreakerMetricsRecorder(provider)
	_ = recorder
}
