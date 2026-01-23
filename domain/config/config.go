// Package config provides domain models for agent configuration.
package config

import "time"

// AgentConfig represents the complete agent configuration.
type AgentConfig struct {
	// Name is a human-readable name for this configuration.
	Name string `json:"name" yaml:"name"`
	// Version is the configuration schema version.
	Version string `json:"version" yaml:"version"`
	// Description describes the agent's purpose.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Agent contains core agent settings.
	Agent AgentSettings `json:"agent" yaml:"agent"`
	// Tools contains tool configurations.
	Tools ToolsConfig `json:"tools,omitempty" yaml:"tools,omitempty"`
	// Policy contains policy settings.
	Policy PolicyConfig `json:"policy,omitempty" yaml:"policy,omitempty"`
	// Resilience contains resilience settings.
	Resilience ResilienceConfig `json:"resilience,omitempty" yaml:"resilience,omitempty"`
	// Notification contains notification settings.
	Notification NotificationConfig `json:"notification,omitempty" yaml:"notification,omitempty"`
	// Variables contains initial variables.
	Variables map[string]any `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// AgentSettings contains core agent behavior settings.
type AgentSettings struct {
	// MaxSteps is the maximum number of execution steps.
	MaxSteps int `json:"max_steps,omitempty" yaml:"max_steps,omitempty"`
	// DefaultGoal is the default goal if none is provided.
	DefaultGoal string `json:"default_goal,omitempty" yaml:"default_goal,omitempty"`
	// InitialState is the starting state (default: intake).
	InitialState string `json:"initial_state,omitempty" yaml:"initial_state,omitempty"`
}

// ToolsConfig contains tool-related configuration.
type ToolsConfig struct {
	// Packs is a list of tool packs to load.
	Packs []ToolPackConfig `json:"packs,omitempty" yaml:"packs,omitempty"`
	// Inline contains inline tool definitions.
	Inline []InlineToolConfig `json:"inline,omitempty" yaml:"inline,omitempty"`
	// Eligibility maps states to allowed tools.
	Eligibility map[string][]string `json:"eligibility,omitempty" yaml:"eligibility,omitempty"`
}

// ToolPackConfig configures a tool pack.
type ToolPackConfig struct {
	// Name is the pack name.
	Name string `json:"name" yaml:"name"`
	// Version is the required version (optional).
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Config contains pack-specific configuration.
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
	// Enabled specifies which tools to enable (empty = all).
	Enabled []string `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Disabled specifies which tools to disable.
	Disabled []string `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

// InlineToolConfig defines an inline tool.
type InlineToolConfig struct {
	// Name is the tool identifier.
	Name string `json:"name" yaml:"name"`
	// Description describes the tool.
	Description string `json:"description" yaml:"description"`
	// Annotations configure tool behavior.
	Annotations ToolAnnotationsConfig `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	// InputSchema is the JSON schema for input validation.
	InputSchema map[string]any `json:"input_schema,omitempty" yaml:"input_schema,omitempty"`
	// OutputSchema is the JSON schema for output validation.
	OutputSchema map[string]any `json:"output_schema,omitempty" yaml:"output_schema,omitempty"`
	// Handler specifies how to execute the tool.
	Handler ToolHandlerConfig `json:"handler" yaml:"handler"`
}

// ToolAnnotationsConfig configures tool annotations.
type ToolAnnotationsConfig struct {
	// ReadOnly indicates the tool doesn't modify state.
	ReadOnly bool `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	// Destructive indicates the tool performs irreversible operations.
	Destructive bool `json:"destructive,omitempty" yaml:"destructive,omitempty"`
	// Idempotent indicates repeated calls produce the same result.
	Idempotent bool `json:"idempotent,omitempty" yaml:"idempotent,omitempty"`
	// Cacheable indicates results can be cached.
	Cacheable bool `json:"cacheable,omitempty" yaml:"cacheable,omitempty"`
	// RiskLevel is the potential impact (none, low, medium, high, critical).
	RiskLevel string `json:"risk_level,omitempty" yaml:"risk_level,omitempty"`
}

// ToolHandlerConfig specifies how to execute a tool.
type ToolHandlerConfig struct {
	// Type is the handler type (http, exec, wasm).
	Type string `json:"type" yaml:"type"`
	// URL is the endpoint for HTTP handlers.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Method is the HTTP method (default: POST).
	Method string `json:"method,omitempty" yaml:"method,omitempty"`
	// Headers are additional HTTP headers.
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Command is the command for exec handlers.
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
	// Args are command arguments for exec handlers.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
	// Env are environment variables for exec handlers.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	// Path is the WASM module path for wasm handlers.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// PolicyConfig contains policy settings.
type PolicyConfig struct {
	// Budgets maps budget names to limits.
	Budgets map[string]int `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	// Approval configures approval behavior.
	Approval ApprovalConfig `json:"approval,omitempty" yaml:"approval,omitempty"`
	// Transitions defines custom state transitions.
	Transitions []TransitionConfig `json:"transitions,omitempty" yaml:"transitions,omitempty"`
	// RateLimit configures rate limiting.
	RateLimit RateLimitConfig `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
}

// ApprovalConfig configures approval behavior.
type ApprovalConfig struct {
	// Mode is the approval mode (auto, manual, none).
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
	// RequireForDestructive requires approval for destructive tools.
	RequireForDestructive bool `json:"require_for_destructive,omitempty" yaml:"require_for_destructive,omitempty"`
	// RequireForRiskLevel requires approval above this risk level.
	RequireForRiskLevel string `json:"require_for_risk_level,omitempty" yaml:"require_for_risk_level,omitempty"`
}

// TransitionConfig defines a state transition.
type TransitionConfig struct {
	// From is the source state.
	From string `json:"from" yaml:"from"`
	// To is the target state.
	To string `json:"to" yaml:"to"`
	// Guard is an optional guard condition.
	Guard string `json:"guard,omitempty" yaml:"guard,omitempty"`
}

// RateLimitConfig configures rate limiting.
type RateLimitConfig struct {
	// Enabled enables rate limiting.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Rate is the tokens per second.
	Rate int `json:"rate,omitempty" yaml:"rate,omitempty"`
	// Burst is the maximum burst size.
	Burst int `json:"burst,omitempty" yaml:"burst,omitempty"`
	// PerTool enables per-tool rate limiting.
	PerTool bool `json:"per_tool,omitempty" yaml:"per_tool,omitempty"`
	// ToolRates maps tool names to rate/burst.
	ToolRates map[string]ToolRateLimitConfig `json:"tool_rates,omitempty" yaml:"tool_rates,omitempty"`
}

// ToolRateLimitConfig configures per-tool rate limiting.
type ToolRateLimitConfig struct {
	// Rate is tokens per second.
	Rate int `json:"rate" yaml:"rate"`
	// Burst is maximum burst size.
	Burst int `json:"burst" yaml:"burst"`
}

// ResilienceConfig contains resilience settings.
type ResilienceConfig struct {
	// Timeout is the default tool timeout.
	Timeout Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// Retry configures retry behavior.
	Retry RetryConfig `json:"retry,omitempty" yaml:"retry,omitempty"`
	// CircuitBreaker configures circuit breaker behavior.
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker,omitempty" yaml:"circuit_breaker,omitempty"`
	// Bulkhead configures bulkhead behavior.
	Bulkhead BulkheadConfig `json:"bulkhead,omitempty" yaml:"bulkhead,omitempty"`
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// Enabled enables retry.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MaxAttempts is the maximum retry attempts.
	MaxAttempts int `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	// InitialDelay is the first retry delay.
	InitialDelay Duration `json:"initial_delay,omitempty" yaml:"initial_delay,omitempty"`
	// MaxDelay is the maximum delay between retries.
	MaxDelay Duration `json:"max_delay,omitempty" yaml:"max_delay,omitempty"`
	// Multiplier is the backoff multiplier.
	Multiplier float64 `json:"multiplier,omitempty" yaml:"multiplier,omitempty"`
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	// Enabled enables circuit breaker.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Threshold is failures before opening.
	Threshold int `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	// Timeout is how long the circuit stays open.
	Timeout Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// BulkheadConfig configures bulkhead behavior.
type BulkheadConfig struct {
	// Enabled enables bulkhead.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MaxConcurrent is the maximum concurrent executions.
	MaxConcurrent int `json:"max_concurrent,omitempty" yaml:"max_concurrent,omitempty"`
}

// NotificationConfig contains notification settings.
type NotificationConfig struct {
	// Enabled enables notifications.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Endpoints is the list of webhook endpoints.
	Endpoints []EndpointConfig `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	// Batching configures event batching.
	Batching BatchingConfig `json:"batching,omitempty" yaml:"batching,omitempty"`
	// EventFilter filters events globally.
	EventFilter []string `json:"event_filter,omitempty" yaml:"event_filter,omitempty"`
}

// EndpointConfig configures a webhook endpoint.
type EndpointConfig struct {
	// Name is a human-readable name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// URL is the webhook URL.
	URL string `json:"url" yaml:"url"`
	// Enabled enables the endpoint.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Secret is the HMAC signing secret.
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
	// Headers are additional HTTP headers.
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// EventFilter filters events for this endpoint.
	EventFilter []string `json:"event_filter,omitempty" yaml:"event_filter,omitempty"`
}

// BatchingConfig configures event batching.
type BatchingConfig struct {
	// Enabled enables batching.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MaxSize is the maximum batch size.
	MaxSize int `json:"max_size,omitempty" yaml:"max_size,omitempty"`
	// MaxWait is the maximum wait before flushing.
	MaxWait Duration `json:"max_wait,omitempty" yaml:"max_wait,omitempty"`
}

// Duration is a time.Duration that supports JSON/YAML string representation.
type Duration time.Duration

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	// Handle null
	if string(b) == "null" {
		return nil
	}

	// Remove quotes
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	// Parse duration
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}
