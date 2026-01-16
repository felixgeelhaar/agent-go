package planner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// BedrockProvider implements the Provider interface for AWS Bedrock.
type BedrockProvider struct {
	region      string
	modelID     string
	credentials aws.CredentialsProvider
	client      *http.Client
}

// BedrockConfig configures the AWS Bedrock provider.
type BedrockConfig struct {
	Region          string // AWS region (e.g., "us-east-1")
	ModelID         string // e.g., "anthropic.claude-3-sonnet-20240229-v1:0"
	AccessKeyID     string // Optional: AWS access key (uses default credential chain if empty)
	SecretAccessKey string // Optional: AWS secret key
	SessionToken    string // Optional: AWS session token
	Timeout         int    // Timeout in seconds (default: 120)
}

// NewBedrockProvider creates a new AWS Bedrock provider.
func NewBedrockProvider(cfg BedrockConfig) (*BedrockProvider, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120
	}

	var creds aws.CredentialsProvider
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use explicit credentials
		creds = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
	} else {
		// Use default credential chain
		awsCfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		creds = awsCfg.Credentials
	}

	return &BedrockProvider{
		region:      region,
		modelID:     cfg.ModelID,
		credentials: creds,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// Name returns the provider name.
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// bedrockClaudeRequest represents the Claude message format for Bedrock.
type bedrockClaudeRequest struct {
	AnthropicVersion string                 `json:"anthropic_version"`
	MaxTokens        int                    `json:"max_tokens"`
	Messages         []bedrockClaudeMessage `json:"messages"`
	System           string                 `json:"system,omitempty"`
	Temperature      float64                `json:"temperature,omitempty"`
}

type bedrockClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// bedrockClaudeResponse represents the Claude response from Bedrock.
type bedrockClaudeResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []bedrockClaudeContent `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type bedrockClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Complete implements the Provider interface.
func (p *BedrockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Extract system message and user messages
	var system string
	var messages []bedrockClaudeMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			system = msg.Content
			continue
		}
		messages = append(messages, bedrockClaudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	bedrockReq := bedrockClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		Messages:         messages,
		System:           system,
		Temperature:      req.Temperature,
	}

	body, err := json.Marshal(bedrockReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the Bedrock endpoint URL
	modelID := req.Model
	if modelID == "" {
		modelID = p.modelID
	}

	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke", p.region, modelID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Sign the request with AWS Signature V4
	creds, err := p.credentials.Retrieve(ctx)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	hash := sha256Hash(body)
	signer := v4.NewSigner()
	err = signer.SignHTTP(ctx, creds, httpReq, hash, "bedrock", p.region, time.Now())
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to sign request: %w", err)
	}

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
		return CompletionResponse{}, sanitizeProviderError("bedrock", resp.StatusCode, respBody)
	}

	var bedrockResp bedrockClaudeResponse
	if err := json.Unmarshal(respBody, &bedrockResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text content from response
	var content string
	for _, c := range bedrockResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return CompletionResponse{
		ID:    bedrockResp.ID,
		Model: bedrockResp.Model,
		Message: Message{
			Role:    bedrockResp.Role,
			Content: content,
		},
		Usage: Usage{
			PromptTokens:     bedrockResp.Usage.InputTokens,
			CompletionTokens: bedrockResp.Usage.OutputTokens,
			TotalTokens:      bedrockResp.Usage.InputTokens + bedrockResp.Usage.OutputTokens,
		},
	}, nil
}

// sha256Hash computes the SHA256 hash of data as a hex string.
func sha256Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
