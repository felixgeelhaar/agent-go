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

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// OllamaConfig configures the Ollama provider.
type OllamaConfig struct {
	BaseURL string // Default: http://localhost:11434
	Model   string // e.g., "llama3.2", "mistral", "codellama"
	Timeout int    // Timeout in seconds (default: 120)
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(config OllamaConfig) *OllamaProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120
	}

	return &OllamaProvider{
		baseURL: baseURL,
		model:   config.Model,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ollamaChatRequest represents the Ollama chat API request.
type ollamaChatRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  *ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaChatResponse represents the Ollama chat API response.
type ollamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// Complete implements the Provider interface.
func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, len(req.Messages))
	for i, msg := range req.Messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Use model from request or fallback to provider default
	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   false,
		Options: &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
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
		return CompletionResponse{}, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return CompletionResponse{
		Model: ollamaResp.Model,
		Message: Message{
			Role:    ollamaResp.Message.Role,
			Content: ollamaResp.Message.Content,
		},
	}, nil
}
