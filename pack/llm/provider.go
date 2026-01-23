// Package llm provides tools for interacting with Large Language Models.
package llm

import (
	"context"
	"errors"
	"time"
)

// Common errors for LLM operations.
var (
	ErrProviderNotConfigured = errors.New("LLM provider not configured")
	ErrRateLimited           = errors.New("rate limited by provider")
	ErrContextTooLong        = errors.New("context too long for model")
	ErrInvalidModel          = errors.New("invalid or unavailable model")
	ErrInvalidInput          = errors.New("invalid input")
	ErrProviderUnavailable   = errors.New("provider unavailable")
)

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Complete generates a completion for the given prompt.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)

	// Classify classifies text into the given labels.
	Classify(ctx context.Context, req ClassifyRequest) (ClassifyResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// CompletionRequest represents a request for text completion.
type CompletionRequest struct {
	// Model is the model to use (e.g., "gpt-4", "claude-3-opus").
	Model string `json:"model"`

	// Prompt is the input prompt.
	Prompt string `json:"prompt"`

	// SystemPrompt is an optional system message.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0 to 2.0).
	Temperature float64 `json:"temperature,omitempty"`

	// TopP is nucleus sampling parameter.
	TopP float64 `json:"top_p,omitempty"`

	// Stop sequences to end generation.
	Stop []string `json:"stop,omitempty"`

	// Stream enables streaming responses.
	Stream bool `json:"stream,omitempty"`
}

// CompletionResponse represents a completion result.
type CompletionResponse struct {
	// Text is the generated text.
	Text string `json:"text"`

	// Usage contains token usage information.
	Usage Usage `json:"usage"`

	// Model is the model that was used.
	Model string `json:"model"`

	// FinishReason indicates why generation stopped.
	FinishReason string `json:"finish_reason"`

	// Latency is the request duration.
	Latency time.Duration `json:"latency"`
}

// Usage represents token usage information.
type Usage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens generated.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the total token count.
	TotalTokens int `json:"total_tokens"`

	// Cost is the estimated cost in USD.
	Cost float64 `json:"cost,omitempty"`
}

// EmbedRequest represents a request for text embeddings.
type EmbedRequest struct {
	// Model is the embedding model to use.
	Model string `json:"model"`

	// Texts are the texts to embed.
	Texts []string `json:"texts"`

	// Dimensions is the desired embedding dimensions (if supported).
	Dimensions int `json:"dimensions,omitempty"`
}

// EmbedResponse represents embedding results.
type EmbedResponse struct {
	// Embeddings are the generated embeddings.
	Embeddings [][]float64 `json:"embeddings"`

	// Model is the model that was used.
	Model string `json:"model"`

	// Dimensions is the embedding dimension.
	Dimensions int `json:"dimensions"`

	// Usage contains token usage information.
	Usage Usage `json:"usage"`
}

// ClassifyRequest represents a text classification request.
type ClassifyRequest struct {
	// Model is the model to use.
	Model string `json:"model"`

	// Text is the text to classify.
	Text string `json:"text"`

	// Labels are the possible classification labels.
	Labels []string `json:"labels"`

	// MultiLabel allows multiple labels to be assigned.
	MultiLabel bool `json:"multi_label,omitempty"`
}

// ClassifyResponse represents classification results.
type ClassifyResponse struct {
	// Classifications are the label scores.
	Classifications []Classification `json:"classifications"`

	// Model is the model that was used.
	Model string `json:"model"`

	// Usage contains token usage information.
	Usage Usage `json:"usage"`
}

// Classification represents a single classification result.
type Classification struct {
	// Label is the classification label.
	Label string `json:"label"`

	// Score is the confidence score (0.0 to 1.0).
	Score float64 `json:"score"`
}
