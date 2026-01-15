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

// GeminiProvider implements the Provider interface for Google Gemini.
type GeminiProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// GeminiConfig configures the Gemini provider.
type GeminiConfig struct {
	APIKey  string // Required: Google AI API key
	BaseURL string // Default: https://generativelanguage.googleapis.com
	Model   string // e.g., "gemini-1.5-pro", "gemini-1.5-flash", "gemini-2.0-flash"
	Timeout int    // Timeout in seconds (default: 120)
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(config GeminiConfig) *GeminiProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120
	}

	return &GeminiProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		model:   config.Model,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// geminiRequest represents the Gemini generateContent API request.
type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// geminiResponse represents the Gemini generateContent API response.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Complete implements the Provider interface.
func (p *GeminiProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Convert messages to Gemini format
	var contents []geminiContent
	var systemInstruction *geminiContent

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
			continue
		}

		// Map roles: assistant -> model, user -> user
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	// Use model from request or fallback to provider default
	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Gemini uses model name in URL path
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		return CompletionResponse{}, fmt.Errorf("gemini error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return CompletionResponse{
			Error: &APIError{
				Type:    geminiResp.Error.Status,
				Message: geminiResp.Error.Message,
				Code:    fmt.Sprintf("%d", geminiResp.Error.Code),
			},
		}, nil
	}

	if len(geminiResp.Candidates) == 0 {
		return CompletionResponse{}, fmt.Errorf("no candidates in response")
	}

	candidate := geminiResp.Candidates[0]

	// Extract text from parts
	var content string
	for _, part := range candidate.Content.Parts {
		content += part.Text
	}

	// Map role back: model -> assistant
	role := candidate.Content.Role
	if role == "model" {
		role = "assistant"
	}

	return CompletionResponse{
		Model: model,
		Message: Message{
			Role:    role,
			Content: content,
		},
		Usage: Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}
