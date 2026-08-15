// Package llm provides LLM completion tools for agent-go.
//
// Tools wrap a Completer (chat/completion). planner-llm providers can be
// adapted with AdaptPlannerProvider. Optional EmbeddingProvider enables
// llm_embed; without it the embed tool returns a configuration error.
//
// Summarize / extract / classify / translate are prompt wrappers over Complete.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage reports token usage when the backend provides it.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// CompleteOptions configures a completion call.
type CompleteOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

// CompleteResult is the model response.
type CompleteResult struct {
	Content string
	Model   string
	Usage   Usage
}

// Completer generates chat completions. Compatible in spirit with
// plannerllm.Provider — use AdaptPlannerProvider to wrap one.
type Completer interface {
	Complete(ctx context.Context, messages []Message, opts CompleteOptions) (CompleteResult, error)
	Name() string
}

// EmbeddingProvider generates embeddings for llm_embed.
type EmbeddingProvider interface {
	EmbedText(ctx context.Context, text string, model string) ([]float64, error)
}

// Config configures the LLM tool pack.
type Config struct {
	Completer  Completer
	Embeddings EmbeddingProvider // optional
	Model      string
	MaxTokens  int
}

// Pack returns the LLM tools pack. cfg.Completer is required.
func Pack(cfg Config) *pack.Pack {
	if cfg.Completer == nil {
		panic("llm.Pack: Completer is required")
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1024
	}
	p := &llmPack{cfg: cfg}
	return pack.NewBuilder("llm").
		WithDescription("LLM completion and text processing tools").
		WithVersion("0.1.0").
		AddTools(
			p.llmComplete(),
			p.llmChat(),
			p.llmEmbed(),
			p.llmSummarize(),
			p.llmExtract(),
			p.llmClassify(),
			p.llmTranslate(),
		).
		AllowInState(agent.StateExplore, "llm_complete", "llm_chat", "llm_embed", "llm_summarize", "llm_extract", "llm_classify", "llm_translate").
		AllowInState(agent.StateAct, "llm_complete", "llm_chat", "llm_embed", "llm_summarize", "llm_extract", "llm_classify", "llm_translate").
		AllowInState(agent.StateDecide, "llm_complete", "llm_chat").
		Build()
}

type llmPack struct {
	cfg Config
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decodeInput[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) == 0 {
		return v, nil
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *llmPack) opts(model string, temperature float64, maxTokens int) CompleteOptions {
	o := CompleteOptions{
		Model:       p.cfg.Model,
		Temperature: temperature,
		MaxTokens:   p.cfg.MaxTokens,
	}
	if model != "" {
		o.Model = model
	}
	if maxTokens > 0 {
		o.MaxTokens = maxTokens
	}
	return o
}

func (p *llmPack) llmComplete() tool.Tool {
	return tool.NewBuilder("llm_complete").
		WithDescription("Generate text completion from a prompt").
		ReadOnly().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"prompt":      json.RawMessage(`{"type":"string"}`),
			"system":      json.RawMessage(`{"type":"string"}`),
			"model":       json.RawMessage(`{"type":"string"}`),
			"temperature": json.RawMessage(`{"type":"number"}`),
			"max_tokens":  json.RawMessage(`{"type":"integer"}`),
		}, []string{"prompt"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Prompt      string  `json:"prompt"`
				System      string  `json:"system"`
				Model       string  `json:"model"`
				Temperature float64 `json:"temperature"`
				MaxTokens   int     `json:"max_tokens"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if strings.TrimSpace(in.Prompt) == "" {
				return tool.Result{}, fmt.Errorf("%w: prompt is required", tool.ErrInvalidInput)
			}
			msgs := make([]Message, 0, 2)
			if in.System != "" {
				msgs = append(msgs, Message{Role: "system", Content: in.System})
			}
			msgs = append(msgs, Message{Role: "user", Content: in.Prompt})
			res, err := p.cfg.Completer.Complete(ctx, msgs, p.opts(in.Model, in.Temperature, in.MaxTokens))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"content":  res.Content,
				"model":    res.Model,
				"provider": p.cfg.Completer.Name(),
				"usage":    res.Usage,
			})
		}).
		MustBuild()
}

func (p *llmPack) llmChat() tool.Tool {
	return tool.NewBuilder("llm_chat").
		WithDescription("Have a multi-turn conversation with an LLM").
		ReadOnly().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"messages":    json.RawMessage(`{"type":"array"}`),
			"model":       json.RawMessage(`{"type":"string"}`),
			"temperature": json.RawMessage(`{"type":"number"}`),
			"max_tokens":  json.RawMessage(`{"type":"integer"}`),
		}, []string{"messages"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Messages    []Message `json:"messages"`
				Model       string    `json:"model"`
				Temperature float64   `json:"temperature"`
				MaxTokens   int       `json:"max_tokens"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if len(in.Messages) == 0 {
				return tool.Result{}, fmt.Errorf("%w: messages are required", tool.ErrInvalidInput)
			}
			res, err := p.cfg.Completer.Complete(ctx, in.Messages, p.opts(in.Model, in.Temperature, in.MaxTokens))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"message":  Message{Role: "assistant", Content: res.Content},
				"model":    res.Model,
				"provider": p.cfg.Completer.Name(),
				"usage":    res.Usage,
			})
		}).
		MustBuild()
}

func (p *llmPack) llmEmbed() tool.Tool {
	return tool.NewBuilder("llm_embed").
		WithDescription("Generate vector embeddings for text").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"text":  json.RawMessage(`{"type":"string"}`),
			"model": json.RawMessage(`{"type":"string"}`),
		}, []string{"text"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if p.cfg.Embeddings == nil {
				return tool.Result{}, fmt.Errorf("%w: EmbeddingProvider not configured", tool.ErrNoHandler)
			}
			in, err := decodeInput[struct {
				Text  string `json:"text"`
				Model string `json:"model"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			vec, err := p.cfg.Embeddings.EmbedText(ctx, in.Text, in.Model)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"embedding": vec, "dimension": len(vec)})
		}).
		MustBuild()
}

func (p *llmPack) llmSummarize() tool.Tool {
	return tool.NewBuilder("llm_summarize").
		WithDescription("Summarize text content").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"text":  json.RawMessage(`{"type":"string"}`),
			"model": json.RawMessage(`{"type":"string"}`),
		}, []string{"text"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Text  string `json:"text"`
				Model string `json:"model"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if strings.TrimSpace(in.Text) == "" {
				return tool.Result{}, fmt.Errorf("%w: text is required", tool.ErrInvalidInput)
			}
			res, err := p.cfg.Completer.Complete(ctx, []Message{
				{Role: "system", Content: "You summarize text clearly and concisely. Reply with the summary only."},
				{Role: "user", Content: in.Text},
			}, p.opts(in.Model, 0, 0))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"content": res.Content, "model": res.Model, "usage": res.Usage})
		}).
		MustBuild()
}

func (p *llmPack) llmExtract() tool.Tool {
	b := tool.NewBuilder("llm_extract").
		WithDescription("Extract structured data from unstructured text").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"text":   json.RawMessage(`{"type":"string"}`),
			"schema": json.RawMessage(`{"type":"string"}`),
			"model":  json.RawMessage(`{"type":"string"}`),
		}, []string{"text"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Text   string `json:"text"`
				Schema string `json:"schema"`
				Model  string `json:"model"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("%w: text is required", tool.ErrInvalidInput)
			}
			user := in.Text
			if in.Schema != "" {
				user = fmt.Sprintf("Extract JSON matching schema:\n%s\n\nText:\n%s", in.Schema, in.Text)
			}
			res, err := p.cfg.Completer.Complete(ctx, []Message{
				{Role: "system", Content: "Extract structured JSON. Reply with JSON only."},
				{Role: "user", Content: user},
			}, p.opts(in.Model, 0, 0))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"content": res.Content, "model": res.Model})
		})
	return b.MustBuild()
}

func (p *llmPack) llmClassify() tool.Tool {
	return tool.NewBuilder("llm_classify").
		WithDescription("Classify text into predefined categories").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"text":       json.RawMessage(`{"type":"string"}`),
			"categories": json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
			"model":      json.RawMessage(`{"type":"string"}`),
		}, []string{"text", "categories"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Text       string   `json:"text"`
				Categories []string `json:"categories"`
				Model      string   `json:"model"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" || len(in.Categories) == 0 {
				return tool.Result{}, fmt.Errorf("%w: text and categories are required", tool.ErrInvalidInput)
			}
			res, err := p.cfg.Completer.Complete(ctx, []Message{
				{Role: "system", Content: "Classify text. Reply with the category name only."},
				{Role: "user", Content: fmt.Sprintf("Categories: %s\n\nText:\n%s", strings.Join(in.Categories, ", "), in.Text)},
			}, p.opts(in.Model, 0, 0))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"category": strings.TrimSpace(res.Content),
				"model":    res.Model,
			})
		}).
		MustBuild()
}

func (p *llmPack) llmTranslate() tool.Tool {
	return tool.NewBuilder("llm_translate").
		WithDescription("Translate text between languages").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"text":        json.RawMessage(`{"type":"string"}`),
			"target_lang": json.RawMessage(`{"type":"string"}`),
			"source_lang": json.RawMessage(`{"type":"string"}`),
			"model":       json.RawMessage(`{"type":"string"}`),
		}, []string{"text", "target_lang"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Text       string `json:"text"`
				TargetLang string `json:"target_lang"`
				SourceLang string `json:"source_lang"`
				Model      string `json:"model"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" || in.TargetLang == "" {
				return tool.Result{}, fmt.Errorf("%w: text and target_lang are required", tool.ErrInvalidInput)
			}
			src := in.SourceLang
			if src == "" {
				src = "auto"
			}
			res, err := p.cfg.Completer.Complete(ctx, []Message{
				{Role: "system", Content: "Translate accurately. Reply with the translation only."},
				{Role: "user", Content: fmt.Sprintf("Translate from %s to %s:\n%s", src, in.TargetLang, in.Text)},
			}, p.opts(in.Model, 0, 0))
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"translation":  res.Content,
				"target_lang":  in.TargetLang,
				"source_lang":  src,
				"model":        res.Model,
			})
		}).
		MustBuild()
}
