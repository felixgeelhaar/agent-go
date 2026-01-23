package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
)

// CopilotProvider implements the Provider interface using GitHub Copilot SDK.
// It provides LLM capabilities through the GitHub Copilot API.
type CopilotProvider struct {
	client  *copilot.Client
	config  CopilotConfig
	session *copilot.Session
	mu      sync.Mutex
}

// CopilotConfig configures the Copilot provider.
type CopilotConfig struct {
	// Model specifies the Copilot model to use (e.g., "gpt-4.1", "gpt-5").
	Model string

	// Streaming enables incremental response reception.
	Streaming bool

	// Timeout is the request timeout in seconds (default: 120).
	Timeout int

	// MaxTokens is the maximum number of tokens to generate (default: 1024).
	MaxTokens int

	// SystemPrompt is an optional system prompt for guiding agent behavior.
	SystemPrompt string

	// CLIPath is the location of the CLI executable.
	// Defaults to "copilot" or the COPILOT_CLI_PATH environment variable.
	CLIPath string

	// CLIUrl is the URL of an existing Copilot CLI server.
	// When set, the client connects to the existing server instead of spawning a new process.
	CLIUrl string

	// Cwd is the working directory for the CLI process.
	Cwd string

	// LogLevel sets logging verbosity (default: "info").
	LogLevel string
}

// NewCopilotProvider creates a new Copilot provider.
func NewCopilotProvider(config CopilotConfig) (*CopilotProvider, error) {
	if config.Model == "" {
		config.Model = "gpt-4.1"
	}
	if config.Timeout == 0 {
		config.Timeout = 120
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 1024
	}
	if config.LogLevel == "" {
		config.LogLevel = "error"
	}

	clientOpts := &copilot.ClientOptions{
		LogLevel: config.LogLevel,
	}

	if config.CLIPath != "" {
		clientOpts.CLIPath = config.CLIPath
	}
	if config.CLIUrl != "" {
		clientOpts.CLIUrl = config.CLIUrl
	}
	if config.Cwd != "" {
		clientOpts.Cwd = config.Cwd
	}

	client := copilot.NewClient(clientOpts)

	return &CopilotProvider{
		client: client,
		config: config,
	}, nil
}

// Name returns the provider name.
func (p *CopilotProvider) Name() string {
	return "copilot"
}

// Start initializes the Copilot client.
func (p *CopilotProvider) Start() error {
	return p.client.Start()
}

// Stop shuts down the Copilot client.
func (p *CopilotProvider) Stop() []error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session != nil {
		_ = p.session.Destroy()
		p.session = nil
	}

	return p.client.Stop()
}

// Complete sends a chat completion request and returns the response.
func (p *CopilotProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create a new session for this request
	sessionConfig := &copilot.SessionConfig{
		Model:     p.getModel(req.Model),
		Streaming: p.config.Streaming,
	}

	session, err := p.client.CreateSession(sessionConfig)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Destroy() }()

	// Build the prompt from messages
	prompt, err := p.buildPrompt(req.Messages)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Set up response collection
	var response copilotResponse
	done := make(chan struct{})
	var responseErr error

	// Subscribe to session events
	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case copilot.AssistantMessage:
			if event.Data.Content != nil {
				response.Content = *event.Data.Content
			}
		case copilot.AssistantMessageDelta:
			if event.Data.DeltaContent != nil {
				response.Content += *event.Data.DeltaContent
			}
		case copilot.SessionIdle:
			close(done)
		case copilot.SessionError:
			if event.Data.Message != nil {
				responseErr = errors.New(*event.Data.Message)
			} else {
				responseErr = errors.New("unknown session error")
			}
			close(done)
		}
	})
	defer unsubscribe()

	// Send the message
	_, err = session.Send(copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for completion with timeout
	timeout := time.Duration(p.config.Timeout) * time.Second
	select {
	case <-done:
		if responseErr != nil {
			return CompletionResponse{}, responseErr
		}
	case <-time.After(timeout):
		return CompletionResponse{}, errors.New("request timed out")
	case <-ctx.Done():
		_ = session.Abort()
		return CompletionResponse{}, ctx.Err()
	}

	// Convert to CompletionResponse
	return CompletionResponse{
		Model: p.getModel(req.Model),
		Message: Message{
			Role:    "assistant",
			Content: response.Content,
		},
	}, nil
}

// copilotResponse holds the accumulated response from Copilot.
type copilotResponse struct {
	Content   string
	Reasoning string
}

// getModel returns the model to use, with fallback to configured default.
func (p *CopilotProvider) getModel(reqModel string) string {
	if reqModel != "" {
		return reqModel
	}
	return p.config.Model
}

// buildPrompt converts messages to a single prompt string.
// The Copilot SDK uses a prompt-based interface rather than message arrays.
func (p *CopilotProvider) buildPrompt(messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("no messages provided")
	}

	var prompt string

	// Include system prompt if configured
	if p.config.SystemPrompt != "" {
		prompt = "System: " + p.config.SystemPrompt + "\n\n"
	}

	// Process messages into a conversation format
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// System messages are prepended
			if prompt == "" {
				prompt = "System: " + msg.Content + "\n\n"
			} else {
				prompt = "System: " + msg.Content + "\n\n" + prompt
			}
		case "user":
			prompt += "User: " + msg.Content + "\n\n"
		case "assistant":
			prompt += "Assistant: " + msg.Content + "\n\n"
		case "tool":
			// Tool results are formatted as system context
			prompt += fmt.Sprintf("Tool Result (%s): %s\n\n", msg.Name, msg.Content)
		}
	}

	return prompt, nil
}

// CompleteWithTools sends a completion request with tool definitions.
// This enables the Copilot SDK's function calling capabilities.
func (p *CopilotProvider) CompleteWithTools(ctx context.Context, req CompletionRequest, tools []copilot.Tool) (CompletionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create session with tools
	sessionConfig := &copilot.SessionConfig{
		Model:     p.getModel(req.Model),
		Streaming: p.config.Streaming,
		Tools:     tools,
	}

	session, err := p.client.CreateSession(sessionConfig)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Destroy() }()

	// Build the prompt
	prompt, err := p.buildPrompt(req.Messages)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Set up response collection with tool call handling
	var response copilotResponse
	var toolCalls []ToolCall
	done := make(chan struct{})
	var responseErr error

	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case copilot.AssistantMessage:
			if event.Data.Content != nil {
				response.Content = *event.Data.Content
			}
		case copilot.ToolExecutionStart:
			// Handle tool execution start events
			if event.Data.ToolName != nil {
				argsJSON, _ := json.Marshal(event.Data.Arguments)
				callID := ""
				if event.Data.ToolCallID != nil {
					callID = *event.Data.ToolCallID
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:   callID,
					Type: "function",
					Function: FunctionCall{
						Name:      *event.Data.ToolName,
						Arguments: string(argsJSON),
					},
				})
			}
		case copilot.SessionIdle:
			close(done)
		case copilot.SessionError:
			if event.Data.Message != nil {
				responseErr = errors.New(*event.Data.Message)
			} else {
				responseErr = errors.New("unknown session error")
			}
			close(done)
		}
	})
	defer unsubscribe()

	// Send the message
	_, err = session.Send(copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for completion
	timeout := time.Duration(p.config.Timeout) * time.Second
	select {
	case <-done:
		if responseErr != nil {
			return CompletionResponse{}, responseErr
		}
	case <-time.After(timeout):
		return CompletionResponse{}, errors.New("request timed out")
	case <-ctx.Done():
		_ = session.Abort()
		return CompletionResponse{}, ctx.Err()
	}

	return CompletionResponse{
		Model: p.getModel(req.Model),
		Message: Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: toolCalls,
		},
	}, nil
}

// CreateSession creates a new Copilot session for interactive use.
// The caller is responsible for calling Destroy() on the session when done.
func (p *CopilotProvider) CreateSession(config *copilot.SessionConfig) (*copilot.Session, error) {
	if config == nil {
		config = &copilot.SessionConfig{
			Model:     p.config.Model,
			Streaming: p.config.Streaming,
		}
	}
	return p.client.CreateSession(config)
}

// GetClient returns the underlying Copilot client for advanced use cases.
func (p *CopilotProvider) GetClient() *copilot.Client {
	return p.client
}
