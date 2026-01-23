package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Config configures the copilot pack.
type Config struct {
	// Provider is the Copilot provider (required).
	Provider Provider

	// Timeout for operations.
	Timeout time.Duration

	// EnableReview enables the code review tool.
	EnableReview bool

	// EnableRefactor enables the refactoring tool.
	EnableRefactor bool

	// DefaultLanguage is used when language is not specified.
	DefaultLanguage string
}

func completeTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_complete").
		WithDescription("Generate code completion using GitHub Copilot").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req CompleteRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.Complete(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("completion failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func explainTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_explain").
		WithDescription("Explain what a piece of code does in natural language").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req ExplainRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			if req.DetailLevel == "" {
				req.DetailLevel = "detailed"
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.Explain(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("explanation failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func reviewTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_review").
		WithDescription("Review code for issues, security vulnerabilities, and improvements").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req ReviewRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			if req.Focus == "" {
				req.Focus = "all"
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.Review(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("review failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func suggestFixTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_suggest_fix").
		WithDescription("Suggest a fix for an error or issue in code").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req SuggestFixRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Error == "" {
				return tool.Result{}, errors.New("error description is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.SuggestFix(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("fix suggestion failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func generateTestsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_generate_tests").
		WithDescription("Generate unit tests for code").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req GenerateTestsRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			if req.Coverage == "" {
				req.Coverage = "unit"
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.GenerateTests(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("test generation failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func refactorTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("copilot_refactor").
		WithDescription("Suggest refactoring improvements for code").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var req RefactorRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if req.Code == "" {
				return tool.Result{}, errors.New("code is required")
			}

			if req.Language == "" {
				req.Language = cfg.DefaultLanguage
			}

			if req.Goal == "" {
				req.Goal = "readability"
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			resp, err := cfg.Provider.Refactor(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("refactoring failed: %w", err)
			}

			data, _ := json.Marshal(resp)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
