package resilience

import "time"

// Option configures the executor.
type Option func(*ExecutorConfig)

// WithMaxConcurrent sets the maximum concurrent executions.
func WithMaxConcurrent(n int) Option {
	return func(c *ExecutorConfig) {
		c.MaxConcurrent = n
	}
}

// WithCircuitBreakerThreshold sets the failure threshold for circuit breaker.
func WithCircuitBreakerThreshold(n int) Option {
	return func(c *ExecutorConfig) {
		c.CircuitBreakerThreshold = n
	}
}

// WithCircuitBreakerTimeout sets the circuit breaker open duration.
func WithCircuitBreakerTimeout(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.CircuitBreakerTimeout = d
	}
}

// WithRetryAttempts sets the maximum retry attempts.
func WithRetryAttempts(n int) Option {
	return func(c *ExecutorConfig) {
		c.RetryMaxAttempts = n
	}
}

// WithRetryDelay sets the initial retry delay.
func WithRetryDelay(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.RetryInitialDelay = d
	}
}

// WithTimeout sets the default execution timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.DefaultTimeout = d
	}
}

// NewExecutorWithOptions creates an executor with the given options.
func NewExecutorWithOptions(opts ...Option) *Executor {
	config := DefaultExecutorConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return NewExecutor(config)
}
