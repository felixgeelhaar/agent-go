package llm

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the LLM pack.
type PackConfig struct {
	// Provider is the LLM provider to use.
	Provider Provider

	// DefaultModel is the default model for completions.
	DefaultModel string

	// DefaultEmbeddingModel is the default model for embeddings.
	DefaultEmbeddingModel string

	// MaxTokensDefault is the default max tokens for completions.
	MaxTokensDefault int

	// TemperatureDefault is the default temperature for completions.
	TemperatureDefault float64
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		MaxTokensDefault:   1024,
		TemperatureDefault: 0.7,
	}
}

// New creates a new LLM pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.MaxTokensDefault == 0 {
		cfg.MaxTokensDefault = 1024
	}
	if cfg.TemperatureDefault == 0 {
		cfg.TemperatureDefault = 0.7
	}

	return pack.NewBuilder("llm").
		WithDescription("Tools for interacting with Large Language Models").
		WithVersion("1.0.0").
		AddTools(
			completeTool(cfg),
			embedTool(cfg),
			classifyTool(cfg),
		).
		AllowInState(agent.StateExplore, "llm_complete", "llm_embed", "llm_classify").
		AllowInState(agent.StateDecide, "llm_complete", "llm_embed", "llm_classify").
		AllowInState(agent.StateAct, "llm_complete", "llm_embed", "llm_classify").
		Build()
}

// completeTool creates the llm_complete tool.
func completeTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("llm_complete").
		WithDescription("Generate text completions using an LLM").
		ReadOnly().
		WithTags("llm", "ai", "generation").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Prompt       string  `json:"prompt"`
				Model        string  `json:"model"`
				SystemPrompt string  `json:"system_prompt"`
				MaxTokens    int     `json:"max_tokens"`
				Temperature  float64 `json:"temperature"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Prompt == "" {
				return tool.Result{}, ErrInvalidInput
			}

			// Apply defaults
			model := req.Model
			if model == "" {
				model = cfg.DefaultModel
			}

			maxTokens := req.MaxTokens
			if maxTokens == 0 {
				maxTokens = cfg.MaxTokensDefault
			}

			temperature := req.Temperature
			if temperature == 0 {
				temperature = cfg.TemperatureDefault
			}

			resp, err := cfg.Provider.Complete(ctx, CompletionRequest{
				Model:        model,
				Prompt:       req.Prompt,
				SystemPrompt: req.SystemPrompt,
				MaxTokens:    maxTokens,
				Temperature:  temperature,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output: output,
			}, nil
		}).
		MustBuild()
}

// embedTool creates the llm_embed tool.
func embedTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("llm_embed").
		WithDescription("Generate embeddings for text").
		ReadOnly().
		Cacheable().
		WithTags("llm", "ai", "embeddings").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Texts      []string `json:"texts"`
				Model      string   `json:"model"`
				Dimensions int      `json:"dimensions"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if len(req.Texts) == 0 {
				return tool.Result{}, ErrInvalidInput
			}

			model := req.Model
			if model == "" {
				model = cfg.DefaultEmbeddingModel
			}

			resp, err := cfg.Provider.Embed(ctx, EmbedRequest{
				Model:      model,
				Texts:      req.Texts,
				Dimensions: req.Dimensions,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output:   output,
				Cached: true,
			}, nil
		}).
		MustBuild()
}

// classifyTool creates the llm_classify tool.
func classifyTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("llm_classify").
		WithDescription("Classify text into predefined labels").
		ReadOnly().
		Cacheable().
		WithTags("llm", "ai", "classification").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Text       string   `json:"text"`
				Labels     []string `json:"labels"`
				Model      string   `json:"model"`
				MultiLabel bool     `json:"multi_label"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Text == "" || len(req.Labels) == 0 {
				return tool.Result{}, ErrInvalidInput
			}

			model := req.Model
			if model == "" {
				model = cfg.DefaultModel
			}

			resp, err := cfg.Provider.Classify(ctx, ClassifyRequest{
				Model:      model,
				Text:       req.Text,
				Labels:     req.Labels,
				MultiLabel: req.MultiLabel,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output:   output,
				Cached: true,
			}, nil
		}).
		MustBuild()
}
