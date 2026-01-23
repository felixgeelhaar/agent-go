// Package copilot provides AI-powered code assistance tools using GitHub Copilot.
package copilot

import (
	"context"
)

// Provider defines the interface for Copilot operations.
// Implementations can use the actual Copilot SDK or mock for testing.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Complete generates code completion for the given context.
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)

	// Explain explains what a piece of code does.
	Explain(ctx context.Context, req ExplainRequest) (ExplainResponse, error)

	// Review analyzes code for potential issues and improvements.
	Review(ctx context.Context, req ReviewRequest) (ReviewResponse, error)

	// SuggestFix suggests a fix for an error or issue.
	SuggestFix(ctx context.Context, req SuggestFixRequest) (SuggestFixResponse, error)

	// GenerateTests generates unit tests for the given code.
	GenerateTests(ctx context.Context, req GenerateTestsRequest) (GenerateTestsResponse, error)

	// Refactor suggests refactoring improvements for code.
	Refactor(ctx context.Context, req RefactorRequest) (RefactorResponse, error)
}

// CompleteRequest is the input for code completion.
type CompleteRequest struct {
	// Code is the code context for completion.
	Code string `json:"code"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
	// Prompt is an optional prompt to guide completion.
	Prompt string `json:"prompt,omitempty"`
	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// CompleteResponse is the output of code completion.
type CompleteResponse struct {
	// Completion is the generated code.
	Completion string `json:"completion"`
	// Confidence is the confidence score (0-1).
	Confidence float64 `json:"confidence,omitempty"`
}

// ExplainRequest is the input for code explanation.
type ExplainRequest struct {
	// Code is the code to explain.
	Code string `json:"code"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
	// DetailLevel controls explanation depth ("brief", "detailed", "comprehensive").
	DetailLevel string `json:"detail_level,omitempty"`
}

// ExplainResponse is the output of code explanation.
type ExplainResponse struct {
	// Explanation is the natural language explanation.
	Explanation string `json:"explanation"`
	// Summary is a brief one-line summary.
	Summary string `json:"summary,omitempty"`
	// KeyConcepts lists important concepts in the code.
	KeyConcepts []string `json:"key_concepts,omitempty"`
}

// ReviewRequest is the input for code review.
type ReviewRequest struct {
	// Code is the code to review.
	Code string `json:"code"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
	// Focus specifies areas to focus on ("security", "performance", "readability", "all").
	Focus string `json:"focus,omitempty"`
}

// ReviewResponse is the output of code review.
type ReviewResponse struct {
	// Issues lists found issues with their severity.
	Issues []Issue `json:"issues"`
	// Suggestions lists improvement suggestions.
	Suggestions []string `json:"suggestions,omitempty"`
	// OverallScore is the code quality score (0-10).
	OverallScore float64 `json:"overall_score,omitempty"`
}

// Issue represents a code issue found during review.
type Issue struct {
	// Severity is the issue severity ("info", "warning", "error", "critical").
	Severity string `json:"severity"`
	// Message describes the issue.
	Message string `json:"message"`
	// Line is the line number where the issue occurs (if applicable).
	Line int `json:"line,omitempty"`
	// Category is the issue category ("security", "performance", "style", "bug").
	Category string `json:"category,omitempty"`
}

// SuggestFixRequest is the input for fix suggestions.
type SuggestFixRequest struct {
	// Code is the code containing the error.
	Code string `json:"code"`
	// Error is the error message or description.
	Error string `json:"error"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
}

// SuggestFixResponse is the output of fix suggestions.
type SuggestFixResponse struct {
	// Fix is the suggested fixed code.
	Fix string `json:"fix"`
	// Explanation describes why this fix works.
	Explanation string `json:"explanation,omitempty"`
	// AlternativeFixes lists other possible fixes.
	AlternativeFixes []string `json:"alternative_fixes,omitempty"`
}

// GenerateTestsRequest is the input for test generation.
type GenerateTestsRequest struct {
	// Code is the code to generate tests for.
	Code string `json:"code"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
	// Framework is the testing framework to use.
	Framework string `json:"framework,omitempty"`
	// Coverage specifies test coverage focus ("unit", "integration", "edge_cases").
	Coverage string `json:"coverage,omitempty"`
}

// GenerateTestsResponse is the output of test generation.
type GenerateTestsResponse struct {
	// Tests is the generated test code.
	Tests string `json:"tests"`
	// TestCases lists the individual test cases generated.
	TestCases []string `json:"test_cases,omitempty"`
	// CoverageEstimate is the estimated coverage percentage.
	CoverageEstimate float64 `json:"coverage_estimate,omitempty"`
}

// RefactorRequest is the input for refactoring suggestions.
type RefactorRequest struct {
	// Code is the code to refactor.
	Code string `json:"code"`
	// Language is the programming language.
	Language string `json:"language,omitempty"`
	// Goal specifies the refactoring goal ("simplify", "performance", "readability", "testability").
	Goal string `json:"goal,omitempty"`
}

// RefactorResponse is the output of refactoring suggestions.
type RefactorResponse struct {
	// RefactoredCode is the refactored version of the code.
	RefactoredCode string `json:"refactored_code"`
	// Changes lists the changes made.
	Changes []string `json:"changes,omitempty"`
	// Improvements describes the benefits of the refactoring.
	Improvements string `json:"improvements,omitempty"`
}
