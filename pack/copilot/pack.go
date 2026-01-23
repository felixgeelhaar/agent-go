package copilot

import (
	"errors"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
)

// Option configures the copilot pack.
type Option func(*Config)

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithReview enables the code review tool.
func WithReview() Option {
	return func(c *Config) {
		c.EnableReview = true
	}
}

// WithRefactor enables the refactoring tool.
func WithRefactor() Option {
	return func(c *Config) {
		c.EnableRefactor = true
	}
}

// WithPRReview enables the PR review tool.
func WithPRReview() Option {
	return func(c *Config) {
		c.EnablePRReview = true
	}
}

// WithIssueAnalysis enables the issue analysis tool.
func WithIssueAnalysis() Option {
	return func(c *Config) {
		c.EnableIssueAnalysis = true
	}
}

// WithDefaultLanguage sets the default programming language.
func WithDefaultLanguage(lang string) Option {
	return func(c *Config) {
		c.DefaultLanguage = lang
	}
}

// New creates the copilot pack.
func New(provider Provider, opts ...Option) (*pack.Pack, error) {
	if provider == nil {
		return nil, errors.New("copilot provider is required")
	}

	cfg := Config{
		Provider:        provider,
		Timeout:         60 * time.Second,
		EnableReview:    false, // Disabled by default as it can be slow
		EnableRefactor:  false, // Disabled by default
		DefaultLanguage: "go",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Core tools - always included
	coreTools := []string{
		"copilot_complete",
		"copilot_explain",
		"copilot_suggest_fix",
		"copilot_generate_tests",
	}

	builder := pack.NewBuilder("copilot").
		WithDescription("AI-powered code assistance using GitHub Copilot").
		WithVersion("1.0.0").
		AddTools(
			completeTool(&cfg),
			explainTool(&cfg),
			suggestFixTool(&cfg),
			generateTestsTool(&cfg),
		)

	// Optional tools
	if cfg.EnableReview {
		builder = builder.AddTools(reviewTool(&cfg))
		coreTools = append(coreTools, "copilot_review")
	}

	if cfg.EnableRefactor {
		builder = builder.AddTools(refactorTool(&cfg))
		coreTools = append(coreTools, "copilot_refactor")
	}

	// GitHub tools
	if cfg.EnablePRReview {
		builder = builder.AddTools(reviewPRTool(&cfg))
		coreTools = append(coreTools, "copilot_review_pr")
	}

	if cfg.EnableIssueAnalysis {
		builder = builder.AddTools(analyzeIssueTool(&cfg))
		coreTools = append(coreTools, "copilot_analyze_issue")
	}

	// All tools are read-only, so they can be used in explore state
	builder = builder.AllowInState(agent.StateExplore, coreTools...)

	// Review tools are also useful in decide state for making decisions
	if cfg.EnableReview {
		builder = builder.AllowInState(agent.StateDecide, "copilot_review")
	}
	if cfg.EnablePRReview {
		builder = builder.AllowInState(agent.StateDecide, "copilot_review_pr")
	}
	if cfg.EnableIssueAnalysis {
		builder = builder.AllowInState(agent.StateDecide, "copilot_analyze_issue")
	}

	// Explain is useful for validation
	builder = builder.AllowInState(agent.StateValidate, "copilot_explain")

	return builder.Build(), nil
}
