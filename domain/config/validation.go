package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	// Path is the JSON path to the invalid field.
	Path string
	// Message describes the validation error.
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("%d validation errors:\n  - %s", len(e), strings.Join(msgs, "\n  - "))
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator validates agent configuration.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the configuration and returns any errors.
func (v *Validator) Validate(config *AgentConfig) ValidationErrors {
	v.errors = nil

	v.validateRequired(config)
	v.validateAgent(config)
	v.validateTools(config)
	v.validatePolicy(config)
	v.validateResilience(config)
	v.validateNotification(config)

	return v.errors
}

func (v *Validator) addError(path, message string) {
	v.errors = append(v.errors, ValidationError{Path: path, Message: message})
}

func (v *Validator) validateRequired(config *AgentConfig) {
	if config.Name == "" {
		v.addError("name", "name is required")
	}
	if config.Version == "" {
		v.addError("version", "version is required")
	}
}

func (v *Validator) validateAgent(config *AgentConfig) {
	if config.Agent.MaxSteps < 0 {
		v.addError("agent.max_steps", "max_steps must be non-negative")
	}

	if config.Agent.InitialState != "" {
		validStates := map[string]bool{
			"intake": true, "explore": true, "decide": true,
			"act": true, "validate": true, "done": true, "failed": true,
		}
		if !validStates[config.Agent.InitialState] {
			v.addError("agent.initial_state", fmt.Sprintf("invalid state: %s", config.Agent.InitialState))
		}
	}
}

func (v *Validator) validateTools(config *AgentConfig) {
	// Validate tool packs
	for i, pack := range config.Tools.Packs {
		path := fmt.Sprintf("tools.packs[%d]", i)
		if pack.Name == "" {
			v.addError(path+".name", "pack name is required")
		}
	}

	// Validate inline tools
	for i, tool := range config.Tools.Inline {
		path := fmt.Sprintf("tools.inline[%d]", i)
		if tool.Name == "" {
			v.addError(path+".name", "tool name is required")
		}
		if tool.Description == "" {
			v.addError(path+".description", "tool description is required")
		}
		if tool.Handler.Type == "" {
			v.addError(path+".handler.type", "handler type is required")
		} else {
			v.validateToolHandler(path+".handler", tool.Handler)
		}
		if tool.Annotations.RiskLevel != "" {
			validLevels := map[string]bool{
				"none": true, "low": true, "medium": true, "high": true, "critical": true,
			}
			if !validLevels[strings.ToLower(tool.Annotations.RiskLevel)] {
				v.addError(path+".annotations.risk_level", fmt.Sprintf("invalid risk level: %s", tool.Annotations.RiskLevel))
			}
		}
	}

	// Validate eligibility
	validStates := map[string]bool{
		"intake": true, "explore": true, "decide": true,
		"act": true, "validate": true,
	}
	for state := range config.Tools.Eligibility {
		if !validStates[state] {
			v.addError(fmt.Sprintf("tools.eligibility.%s", state), fmt.Sprintf("invalid state: %s", state))
		}
	}
}

func (v *Validator) validateToolHandler(path string, handler ToolHandlerConfig) {
	switch handler.Type {
	case "http":
		if handler.URL == "" {
			v.addError(path+".url", "URL is required for http handler")
		}
	case "exec":
		if handler.Command == "" {
			v.addError(path+".command", "command is required for exec handler")
		}
	case "wasm":
		if handler.Path == "" {
			v.addError(path+".path", "path is required for wasm handler")
		}
	default:
		v.addError(path+".type", fmt.Sprintf("unknown handler type: %s", handler.Type))
	}
}

func (v *Validator) validatePolicy(config *AgentConfig) {
	// Validate budgets
	for name, limit := range config.Policy.Budgets {
		if limit < 0 {
			v.addError(fmt.Sprintf("policy.budgets.%s", name), "budget limit must be non-negative")
		}
	}

	// Validate approval mode
	if config.Policy.Approval.Mode != "" {
		validModes := map[string]bool{
			"auto": true, "manual": true, "none": true,
		}
		if !validModes[config.Policy.Approval.Mode] {
			v.addError("policy.approval.mode", fmt.Sprintf("invalid mode: %s", config.Policy.Approval.Mode))
		}
	}

	// Validate approval risk level
	if config.Policy.Approval.RequireForRiskLevel != "" {
		validLevels := map[string]bool{
			"none": true, "low": true, "medium": true, "high": true, "critical": true,
		}
		if !validLevels[strings.ToLower(config.Policy.Approval.RequireForRiskLevel)] {
			v.addError("policy.approval.require_for_risk_level",
				fmt.Sprintf("invalid risk level: %s", config.Policy.Approval.RequireForRiskLevel))
		}
	}

	// Validate transitions
	validStates := map[string]bool{
		"intake": true, "explore": true, "decide": true,
		"act": true, "validate": true, "done": true, "failed": true,
	}
	for i, trans := range config.Policy.Transitions {
		path := fmt.Sprintf("policy.transitions[%d]", i)
		if trans.From == "" {
			v.addError(path+".from", "from state is required")
		} else if !validStates[trans.From] {
			v.addError(path+".from", fmt.Sprintf("invalid state: %s", trans.From))
		}
		if trans.To == "" {
			v.addError(path+".to", "to state is required")
		} else if !validStates[trans.To] {
			v.addError(path+".to", fmt.Sprintf("invalid state: %s", trans.To))
		}
	}

	// Validate rate limit
	if config.Policy.RateLimit.Enabled {
		if config.Policy.RateLimit.Rate <= 0 {
			v.addError("policy.rate_limit.rate", "rate must be positive when enabled")
		}
		if config.Policy.RateLimit.Burst <= 0 {
			v.addError("policy.rate_limit.burst", "burst must be positive when enabled")
		}
	}
}

func (v *Validator) validateResilience(config *AgentConfig) {
	// Validate retry
	if config.Resilience.Retry.Enabled {
		if config.Resilience.Retry.MaxAttempts <= 0 {
			v.addError("resilience.retry.max_attempts", "max_attempts must be positive when enabled")
		}
		if config.Resilience.Retry.Multiplier < 1 {
			v.addError("resilience.retry.multiplier", "multiplier must be >= 1")
		}
	}

	// Validate circuit breaker
	if config.Resilience.CircuitBreaker.Enabled {
		if config.Resilience.CircuitBreaker.Threshold <= 0 {
			v.addError("resilience.circuit_breaker.threshold", "threshold must be positive when enabled")
		}
	}

	// Validate bulkhead
	if config.Resilience.Bulkhead.Enabled {
		if config.Resilience.Bulkhead.MaxConcurrent <= 0 {
			v.addError("resilience.bulkhead.max_concurrent", "max_concurrent must be positive when enabled")
		}
	}
}

func (v *Validator) validateNotification(config *AgentConfig) {
	if !config.Notification.Enabled {
		return
	}

	// Validate endpoints
	for i, ep := range config.Notification.Endpoints {
		path := fmt.Sprintf("notification.endpoints[%d]", i)
		if ep.URL == "" {
			v.addError(path+".url", "URL is required")
		}
	}

	// Validate batching
	if config.Notification.Batching.Enabled {
		if config.Notification.Batching.MaxSize <= 0 {
			v.addError("notification.batching.max_size", "max_size must be positive when enabled")
		}
	}
}
