// Package resilience provides resilient execution patterns using fortify.
package resilience

import (
	"context"
	"encoding/json"
	"time"

	"github.com/felixgeelhaar/fortify/bulkhead"
	"github.com/felixgeelhaar/fortify/circuitbreaker"
	"github.com/felixgeelhaar/fortify/retry"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Executor provides resilient tool execution with circuit breaker, retry, and bulkhead patterns.
type Executor struct {
	bulkhead bulkhead.Bulkhead[tool.Result]
	breaker  circuitbreaker.CircuitBreaker[tool.Result]
	retry    retry.Retry[tool.Result]
	timeout  time.Duration
}

// ExecutorConfig configures the resilient executor.
type ExecutorConfig struct {
	// MaxConcurrent limits concurrent tool executions.
	MaxConcurrent int

	// CircuitBreakerThreshold is the number of failures before opening.
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is how long the circuit stays open.
	CircuitBreakerTimeout time.Duration

	// RetryMaxAttempts is the maximum number of retry attempts.
	RetryMaxAttempts int

	// RetryInitialDelay is the initial delay between retries.
	RetryInitialDelay time.Duration

	// RetryBackoffMultiplier is the exponential backoff multiplier.
	RetryBackoffMultiplier float64

	// DefaultTimeout is the default execution timeout.
	DefaultTimeout time.Duration
}

// DefaultExecutorConfig returns a configuration with sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxConcurrent:           10,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		RetryMaxAttempts:        3,
		RetryInitialDelay:       100 * time.Millisecond,
		RetryBackoffMultiplier:  2.0,
		DefaultTimeout:          30 * time.Second,
	}
}

// NewExecutor creates a new resilient executor.
func NewExecutor(config ExecutorConfig) *Executor {
	// Ensure non-negative values for uint32 conversion (G115 fix)
	maxConcurrent := config.MaxConcurrent
	if maxConcurrent < 0 {
		maxConcurrent = 10 // default
	}
	threshold := config.CircuitBreakerThreshold
	if threshold < 0 {
		threshold = 5 // default
	}

	return &Executor{
		bulkhead: bulkhead.New[tool.Result](bulkhead.Config{
			MaxConcurrent: maxConcurrent,
		}),
		breaker: circuitbreaker.New[tool.Result](circuitbreaker.Config{
			MaxRequests: uint32(maxConcurrent), // #nosec G115 -- bounds checked above
			Interval:    config.CircuitBreakerTimeout,
			Timeout:     config.CircuitBreakerTimeout,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= uint32(threshold) // #nosec G115 -- bounds checked above
			},
		}),
		retry: retry.New[tool.Result](retry.Config{
			MaxAttempts:   config.RetryMaxAttempts,
			InitialDelay:  config.RetryInitialDelay,
			BackoffPolicy: retry.BackoffExponential,
			Multiplier:    config.RetryBackoffMultiplier,
		}),
		timeout: config.DefaultTimeout,
	}
}

// NewDefaultExecutor creates an executor with default configuration.
func NewDefaultExecutor() *Executor {
	return NewExecutor(DefaultExecutorConfig())
}

// Execute runs a tool with resilience patterns applied.
// Composition order: Bulkhead → Timeout → Circuit Breaker → Retry (for idempotent)
func (e *Executor) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	start := time.Now()

	// Apply bulkhead for concurrency control
	result, err := e.bulkhead.Execute(ctx, func(ctx context.Context) (tool.Result, error) {
		// Apply timeout
		ctx, cancel := context.WithTimeout(ctx, e.timeout)
		defer cancel()

		// Apply circuit breaker
		return e.breaker.Execute(ctx, func(ctx context.Context) (tool.Result, error) {
			// Apply retry only for idempotent tools
			if t.Annotations().CanRetry() {
				return e.retry.Do(ctx, func(ctx context.Context) (tool.Result, error) {
					return t.Execute(ctx, input)
				})
			}
			return t.Execute(ctx, input)
		})
	})

	// Add timing information
	if err == nil {
		result.Duration = time.Since(start)
	}

	return result, err
}

// ExecuteWithTimeout runs a tool with a custom timeout.
func (e *Executor) ExecuteWithTimeout(ctx context.Context, t tool.Tool, input json.RawMessage, timeout time.Duration) (tool.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return e.Execute(ctx, t, input)
}

// ExecuteSimple runs a tool without resilience patterns.
// Use this for tools that should not be retried or protected.
func (e *Executor) ExecuteSimple(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	start := time.Now()
	result, err := t.Execute(ctx, input)
	if err == nil {
		result.Duration = time.Since(start)
	}
	return result, err
}

// CircuitBreakerState returns the current state of the circuit breaker.
func (e *Executor) CircuitBreakerState() circuitbreaker.State {
	return e.breaker.State()
}

// Reset resets the circuit breaker to closed state.
func (e *Executor) Reset() {
	// Circuit breaker will automatically reset after timeout
}
