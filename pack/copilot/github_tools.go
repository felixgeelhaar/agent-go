package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// reviewPRTool creates the copilot_review_pr tool.
func reviewPRTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_review_pr").
		WithDescription("Review a pull request diff and provide feedback on code quality, security, and best practices").
		ReadOnly().
		WithTags("copilot", "github", "pr", "review").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req ReviewPRRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Diff == "" {
				return tool.Result{}, fmt.Errorf("diff is required")
			}

			// Apply timeout
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = 60 * time.Second
			}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			resp, err := cfg.Provider.ReviewPR(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("PR review failed: %w", err)
			}

			output, err := json.Marshal(resp)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal response: %w", err)
			}

			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// analyzeIssueTool creates the copilot_analyze_issue tool.
func analyzeIssueTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_analyze_issue").
		WithDescription("Analyze a GitHub issue and suggest solutions, categorization, and priority").
		ReadOnly().
		WithTags("copilot", "github", "issue", "analysis").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req AnalyzeIssueRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Title == "" {
				return tool.Result{}, fmt.Errorf("title is required")
			}
			if req.Body == "" {
				return tool.Result{}, fmt.Errorf("body is required")
			}

			// Apply timeout
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			resp, err := cfg.Provider.AnalyzeIssue(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("issue analysis failed: %w", err)
			}

			output, err := json.Marshal(resp)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal response: %w", err)
			}

			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
