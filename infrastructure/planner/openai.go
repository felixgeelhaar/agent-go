package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// OpenAIConfig configures the OpenAI provider.
type OpenAIConfig struct {
	APIKey  string // Required: OpenAI API key
	BaseURL string // Default: https://api.openai.com
	Model   string // e.g., "gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"
	Timeout int    // Timeout in seconds (default: 120)
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config OpenAIConfig) *OpenAIProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120
	}

	return &OpenAIProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		model:   config.Model,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// openAIChatRequest represents the OpenAI chat completions API request.
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatResponse represents the OpenAI chat completions API response.
type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete implements the Provider interface.
func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Convert messages to OpenAI format
	openAIMessages := make([]openAIMessage, len(req.Messages))
	for i, msg := range req.Messages {
		openAIMessages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Use model from request or fallback to provider default
	model := req.Model
	if model == "" {
		model = p.model
	}

	openAIReq := openAIChatRequest{
		Model:       model,
		Messages:    openAIMessages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, fmt.Errorf("openai error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIChatResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Error != nil {
		return CompletionResponse{
			Error: &APIError{
				Type:    openAIResp.Error.Type,
				Message: openAIResp.Error.Message,
				Code:    openAIResp.Error.Code,
			},
		}, nil
	}

	if len(openAIResp.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("no choices in response")
	}

	choice := openAIResp.Choices[0]

	return CompletionResponse{
		ID:    openAIResp.ID,
		Model: openAIResp.Model,
		Message: Message{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
		},
		Usage: Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, nil
}
