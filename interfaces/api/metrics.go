// Package api provides the public API for the agent-go library.
// This file provides metrics and telemetry-related exports.
package api

import (
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	inframw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
	"github.com/felixgeelhaar/agent-go/infrastructure/telemetry"
)

// Re-export telemetry types.
type (
	// MetricsProvider provides access to OpenTelemetry metrics instruments.
	MetricsProvider = telemetry.MetricsProvider

	// MetricsConfig configures the metrics provider.
	MetricsConfig = telemetry.MetricsConfig

	// Metrics is the interface for recording metrics.
	Metrics = telemetry.Metrics

	// NoopMetricsProvider is a no-op implementation for testing.
	NoopMetricsProvider = telemetry.NoopMetricsProvider
)

// Re-export middleware metrics types.
type (
	// MetricsMiddlewareConfig configures the metrics middleware.
	MetricsMiddlewareConfig = inframw.MetricsConfig

	// CacheMetricsRecorder records cache-related metrics.
	CacheMetricsRecorder = inframw.CacheMetricsRecorder

	// RateLimitMetricsRecorder records rate limit metrics.
	RateLimitMetricsRecorder = inframw.RateLimitMetricsRecorder

	// CircuitBreakerMetricsRecorder records circuit breaker metrics.
	CircuitBreakerMetricsRecorder = inframw.CircuitBreakerMetricsRecorder
)

// NewMetricsProvider creates a new OpenTelemetry metrics provider.
//
// The provider records metrics for:
//   - Tool executions (count, duration, success/failure)
//   - State transitions
//   - Budget consumption
//   - Rate limit hits
//   - Cache hits/misses
//   - Active runs
//   - Circuit breaker states
//   - Errors
//
// Example:
//
//	// Create a metrics provider
//	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
//
//	// Check for initialization errors
//	if err := provider.Error(); err != nil {
//	    log.Fatalf("failed to create metrics provider: %v", err)
//	}
//
//	// Use with metrics middleware
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithMetrics(provider),
//	)
func NewMetricsProvider(config MetricsConfig) *MetricsProvider {
	return telemetry.NewMetricsProvider(config)
}

// DefaultMetricsConfig returns the default metrics configuration.
func DefaultMetricsConfig() MetricsConfig {
	return telemetry.DefaultMetricsConfig()
}

// WithMetrics adds metrics middleware to the engine.
//
// This middleware records:
//   - Tool execution count (with tool name, state, and success attributes)
//   - Tool execution duration histogram
//   - Errors (when tool execution fails)
//
// Example:
//
//	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithMetrics(provider),
//	)
func WithMetrics(provider Metrics) Option {
	return func(c *engineConfig) {
		mw := inframw.Metrics(inframw.MetricsConfig{Provider: provider})
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		c.middleware.Use(mw)
	}
}

// NewCacheMetricsRecorder creates a recorder for cache metrics.
// Use this with caching middleware to track cache hits and misses.
//
// Example:
//
//	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
//	cacheRecorder := api.NewCacheMetricsRecorder(provider)
//
//	// In your caching logic:
//	if cached {
//	    cacheRecorder.RecordHit(ctx, toolName)
//	} else {
//	    cacheRecorder.RecordMiss(ctx, toolName)
//	}
func NewCacheMetricsRecorder(provider Metrics) CacheMetricsRecorder {
	return inframw.MetricsWithCaching(inframw.MetricsConfig{Provider: provider})
}

// NewRateLimitMetricsRecorder creates a recorder for rate limit metrics.
//
// Example:
//
//	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
//	rlRecorder := api.NewRateLimitMetricsRecorder(provider)
//
//	// In your rate limiting logic:
//	if rateLimited {
//	    rlRecorder.RecordLimitHit(ctx, toolName)
//	}
func NewRateLimitMetricsRecorder(provider Metrics) RateLimitMetricsRecorder {
	return inframw.MetricsRateLimitRecorder(inframw.MetricsConfig{Provider: provider})
}

// NewCircuitBreakerMetricsRecorder creates a recorder for circuit breaker metrics.
//
// Example:
//
//	provider := api.NewMetricsProvider(api.DefaultMetricsConfig())
//	cbRecorder := api.NewCircuitBreakerMetricsRecorder(provider)
//
//	// In your circuit breaker logic:
//	cbRecorder.RecordStateChange(ctx, toolName, isOpen)
func NewCircuitBreakerMetricsRecorder(provider Metrics) CircuitBreakerMetricsRecorder {
	return inframw.MetricsCircuitBreakerRecorder(inframw.MetricsConfig{Provider: provider})
}
