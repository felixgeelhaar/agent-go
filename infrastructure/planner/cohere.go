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

// CohereProvider implements the Provider interface for Cohere.
type CohereProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// CohereConfig configures the Cohere provider.
type CohereConfig struct {
	APIKey  string // Required: Cohere API key
	BaseURL string // Default: https://api.cohere.ai
	Model   string // e.g., "command-r", "command-r-plus"
	Timeout int    // Timeout in seconds (default: 120)
}

// NewCohereProvider creates a new Cohere provider.
func NewCohereProvider(config CohereConfig) *CohereProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.cohere.ai"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120
	}

	return &CohereProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		model:   config.Model,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *CohereProvider) Name() string {
	return "cohere"
}

// cohereChatRequest represents the Cohere chat API request.
type cohereChatRequest struct {
	Model       string               `json:"model"`
	Message     string               `json:"message"`
	ChatHistory []cohereChatMessage  `json:"chat_history,omitempty"`
	Preamble    string               `json:"preamble,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
}

type cohereChatMessage struct {
	Role    string `json:"role"` // USER, CHATBOT, SYSTEM
	Message string `json:"message"`
}

// cohereChatResponse represents the Cohere chat API response.
type cohereChatResponse struct {
	ResponseID       string `json:"response_id"`
	Text             string `json:"text"`
	GenerationID     string `json:"generation_id"`
	FinishReason     string `json:"finish_reason"`
	Meta             cohereMeta `json:"meta,omitempty"`
	Message          string `json:"message,omitempty"` // Error message
}

type cohereMeta struct {
	APIVersion struct {
		Version string `json:"version"`
	} `json:"api_version,omitempty"`
	BilledUnits struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"billed_units,omitempty"`
	Tokens struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"tokens,omitempty"`
}

// Complete implements the Provider interface.
func (p *CohereProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Extract system message, history, and current user message
	var preamble string
	var chatHistory []cohereChatMessage
	var currentMessage string

	for i, msg := range req.Messages {
		switch msg.Role {
		case "system":
			preamble = msg.Content
		case "user":
			// If this is the last message, it's the current message
			if i == len(req.Messages)-1 {
				currentMessage = msg.Content
			} else {
				chatHistory = append(chatHistory, cohereChatMessage{
					Role:    "USER",
					Message: msg.Content,
				})
			}
		case "assistant":
			chatHistory = append(chatHistory, cohereChatMessage{
				Role:    "CHATBOT",
				Message: msg.Content,
			})
		}
	}

	// Use model from request or fallback to provider default
	model := req.Model
	if model == "" {
		model = p.model
	}

	cohereReq := cohereChatRequest{
		Model:       model,
		Message:     currentMessage,
		ChatHistory: chatHistory,
		Preamble:    preamble,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	body, err := json.Marshal(cohereReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
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
		return CompletionResponse{}, fmt.Errorf("cohere error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var cohereResp cohereChatResponse
	if err := json.Unmarshal(respBody, &cohereResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error in response
	if cohereResp.Message != "" && cohereResp.Text == "" {
		return CompletionResponse{
			Error: &APIError{
				Type:    "cohere_error",
				Message: cohereResp.Message,
			},
		}, nil
	}

	// Get token counts from meta (try both billed_units and tokens)
	inputTokens := cohereResp.Meta.Tokens.InputTokens
	outputTokens := cohereResp.Meta.Tokens.OutputTokens
	if inputTokens == 0 {
		inputTokens = cohereResp.Meta.BilledUnits.InputTokens
		outputTokens = cohereResp.Meta.BilledUnits.OutputTokens
	}

	return CompletionResponse{
		ID:    cohereResp.ResponseID,
		Model: model,
		Message: Message{
			Role:    "assistant",
			Content: cohereResp.Text,
		},
		Usage: Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}, nil
}
