package copilot

import (
	"context"
	"fmt"
	"strings"
)

// MemoryProvider is an in-memory implementation of the Provider interface.
// Useful for testing and development without requiring actual Copilot access.
type MemoryProvider struct {
	name string

	// Configurable responses for testing
	CompleteFunc      func(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
	ExplainFunc       func(ctx context.Context, req ExplainRequest) (ExplainResponse, error)
	ReviewFunc        func(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
	SuggestFixFunc    func(ctx context.Context, req SuggestFixRequest) (SuggestFixResponse, error)
	GenerateTestsFunc func(ctx context.Context, req GenerateTestsRequest) (GenerateTestsResponse, error)
	RefactorFunc      func(ctx context.Context, req RefactorRequest) (RefactorResponse, error)
	ReviewPRFunc      func(ctx context.Context, req ReviewPRRequest) (ReviewPRResponse, error)
	AnalyzeIssueFunc  func(ctx context.Context, req AnalyzeIssueRequest) (AnalyzeIssueResponse, error)
}

// NewMemoryProvider creates a new in-memory Copilot provider with sensible defaults.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		name: "memory",
	}
}

// Name returns the provider name.
func (p *MemoryProvider) Name() string {
	return p.name
}

// Complete generates code completion.
func (p *MemoryProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	if p.CompleteFunc != nil {
		return p.CompleteFunc(ctx, req)
	}

	// Default mock implementation
	return CompleteResponse{
		Completion: fmt.Sprintf("// Generated completion for %s code\n%s\n// ... additional code",
			req.Language, req.Code),
		Confidence: 0.85,
	}, nil
}

// Explain explains code.
func (p *MemoryProvider) Explain(ctx context.Context, req ExplainRequest) (ExplainResponse, error) {
	if p.ExplainFunc != nil {
		return p.ExplainFunc(ctx, req)
	}

	// Default mock implementation
	lines := strings.Count(req.Code, "\n") + 1
	return ExplainResponse{
		Explanation: fmt.Sprintf("This %s code (%d lines) performs the following operations:\n"+
			"1. Initializes necessary variables\n"+
			"2. Processes the input data\n"+
			"3. Returns the computed result",
			req.Language, lines),
		Summary: fmt.Sprintf("A %s code block that processes data", req.Language),
		KeyConcepts: []string{
			"data processing",
			"control flow",
			"return value",
		},
	}, nil
}

// Review analyzes code for issues.
func (p *MemoryProvider) Review(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	if p.ReviewFunc != nil {
		return p.ReviewFunc(ctx, req)
	}

	// Default mock implementation
	issues := []Issue{}

	// Simple heuristics for mock
	if strings.Contains(req.Code, "TODO") {
		issues = append(issues, Issue{
			Severity: "info",
			Message:  "Found TODO comment that should be addressed",
			Category: "style",
		})
	}

	if strings.Contains(req.Code, "password") || strings.Contains(req.Code, "secret") {
		issues = append(issues, Issue{
			Severity: "warning",
			Message:  "Potential sensitive data handling detected",
			Category: "security",
		})
	}

	score := 8.0
	if len(issues) > 0 {
		score = 8.0 - float64(len(issues))*0.5
	}

	return ReviewResponse{
		Issues: issues,
		Suggestions: []string{
			"Consider adding more comments for complex logic",
			"Ensure error handling is comprehensive",
		},
		OverallScore: score,
	}, nil
}

// SuggestFix suggests a fix for an error.
func (p *MemoryProvider) SuggestFix(ctx context.Context, req SuggestFixRequest) (SuggestFixResponse, error) {
	if p.SuggestFixFunc != nil {
		return p.SuggestFixFunc(ctx, req)
	}

	// Default mock implementation
	return SuggestFixResponse{
		Fix: fmt.Sprintf("// Fixed version:\n%s\n// Error '%s' has been addressed",
			req.Code, req.Error),
		Explanation: fmt.Sprintf("The error '%s' was caused by incorrect handling. "+
			"The fix ensures proper validation and error handling.", req.Error),
		AlternativeFixes: []string{
			"Consider using a different approach with better error handling",
		},
	}, nil
}

// GenerateTests generates unit tests.
func (p *MemoryProvider) GenerateTests(ctx context.Context, req GenerateTestsRequest) (GenerateTestsResponse, error) {
	if p.GenerateTestsFunc != nil {
		return p.GenerateTestsFunc(ctx, req)
	}

	// Default mock implementation based on language
	var tests string

	switch req.Language {
	case "go", "golang":
		tests = `func TestFunction(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"basic case", "expected"},
		{"edge case", "expected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test implementation
		})
	}
}`
	case "python":
		tests = `import pytest

def test_function():
    """Test the function."""
    assert True

def test_edge_case():
    """Test edge cases."""
    assert True`
	default:
		tests = fmt.Sprintf("// Test suite for %s code\n// Add test cases here", req.Language)
	}

	return GenerateTestsResponse{
		Tests: tests,
		TestCases: []string{
			"test_basic_functionality",
			"test_edge_cases",
			"test_error_handling",
		},
		CoverageEstimate: 75.0,
	}, nil
}

// Refactor suggests refactoring improvements.
func (p *MemoryProvider) Refactor(ctx context.Context, req RefactorRequest) (RefactorResponse, error) {
	if p.RefactorFunc != nil {
		return p.RefactorFunc(ctx, req)
	}

	// Default mock implementation
	return RefactorResponse{
		RefactoredCode: fmt.Sprintf("// Refactored for %s:\n%s",
			req.Goal, req.Code),
		Changes: []string{
			"Extracted helper functions for better modularity",
			"Improved variable naming for clarity",
			"Simplified control flow",
		},
		Improvements: fmt.Sprintf("The refactored code improves %s by "+
			"reducing complexity and enhancing maintainability.", req.Goal),
	}, nil
}

// ReviewPR reviews a pull request.
func (p *MemoryProvider) ReviewPR(ctx context.Context, req ReviewPRRequest) (ReviewPRResponse, error) {
	if p.ReviewPRFunc != nil {
		return p.ReviewPRFunc(ctx, req)
	}

	// Default mock implementation
	comments := []PRComment{}

	// Generate mock comments based on diff content
	if strings.Contains(req.Diff, "TODO") {
		comments = append(comments, PRComment{
			File:     "unknown",
			Body:     "Found TODO comment that should be addressed before merging",
			Severity: "warning",
		})
	}

	if strings.Contains(req.Diff, "password") || strings.Contains(req.Diff, "secret") {
		comments = append(comments, PRComment{
			File:     "unknown",
			Body:     "Potential sensitive data detected - ensure proper handling",
			Severity: "error",
		})
	}

	verdict := "approve"
	riskLevel := "low"
	if len(comments) > 0 {
		verdict = "comment"
		riskLevel = "medium"
	}

	return ReviewPRResponse{
		Summary:   fmt.Sprintf("PR Review: %s\n\nThis PR contains changes that have been reviewed for quality and security.", req.Title),
		Comments:  comments,
		Verdict:   verdict,
		RiskLevel: riskLevel,
	}, nil
}

// AnalyzeIssue analyzes a GitHub issue.
func (p *MemoryProvider) AnalyzeIssue(ctx context.Context, req AnalyzeIssueRequest) (AnalyzeIssueResponse, error) {
	if p.AnalyzeIssueFunc != nil {
		return p.AnalyzeIssueFunc(ctx, req)
	}

	// Default mock implementation - categorize based on title/body content
	category := "feature"
	priority := "medium"
	effort := "medium"

	titleLower := strings.ToLower(req.Title)
	bodyLower := strings.ToLower(req.Body)

	if strings.Contains(titleLower, "bug") || strings.Contains(titleLower, "error") ||
		strings.Contains(titleLower, "fix") || strings.Contains(titleLower, "crash") {
		category = "bug"
		priority = "high"
		effort = "small"
	} else if strings.Contains(titleLower, "question") || strings.Contains(bodyLower, "how do") {
		category = "question"
		priority = "low"
		effort = "trivial"
	} else if strings.Contains(titleLower, "doc") || strings.Contains(titleLower, "readme") {
		category = "documentation"
		priority = "low"
		effort = "small"
	}

	return AnalyzeIssueResponse{
		Summary:           fmt.Sprintf("Issue Analysis: %s\n\nThis issue has been analyzed and categorized.", req.Title),
		Category:          category,
		Priority:          priority,
		SuggestedSolution: "Investigate the reported issue and implement appropriate changes.",
		RelatedAreas:      []string{"core", "api"},
		EstimatedEffort:   effort,
	}, nil
}
