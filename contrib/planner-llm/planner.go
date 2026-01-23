// Package plannerllm provides LLM-based planner implementations for agent-go.
//
// This package wraps the core planner interface to provide integrations with
// various LLM providers including OpenAI, Anthropic, Google Gemini, Cohere,
// AWS Bedrock, and GitHub Copilot.
//
// # Usage
//
//	provider := plannerllm.NewOpenAIProvider(plannerllm.OpenAIConfig{
//		APIKey: os.Getenv("OPENAI_API_KEY"),
//		Model:  "gpt-4",
//	})
//
//	planner := plannerllm.NewPlanner(plannerllm.Config{
//		Provider:    provider,
//		Temperature: 0.7,
//		MaxTokens:   1024,
//	})
//
//	// Use planner with agent engine
//	engine, err := api.New(api.WithPlanner(planner))
package plannerllm

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// Provider defines the interface for LLM providers.
// Each provider implementation handles the specifics of communicating
// with a particular LLM service (OpenAI, Anthropic, etc.).
type Provider interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Name returns the provider name for logging and metrics.
	Name() string
}

// StreamingProvider extends Provider with streaming capabilities.
type StreamingProvider interface {
	Provider

	// CompleteStream sends a streaming completion request.
	// The returned channel receives content chunks until completion.
	CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// CompletionRequest represents a chat completion request.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// CompletionResponse represents a chat completion response.
type CompletionResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Message Message  `json:"message"`
	Usage   Usage    `json:"usage"`
	Error   error    `json:"error,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role      string     `json:"role"` // system, user, assistant, tool
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Tool represents a tool definition for function calling.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ToolCall represents a tool invocation from the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
}

// Planner is the interface that all planner implementations must satisfy.
// It matches the core agent runtime's planner contract.
type Planner interface {
	// Plan makes a planning decision based on the current state.
	Plan(ctx context.Context, req PlanRequest) (agent.Decision, error)
}

// PlanRequest contains all information needed for a planning decision.
type PlanRequest struct {
	RunID        string              `json:"run_id"`
	Goal         string              `json:"goal"`
	CurrentState agent.State         `json:"current_state"`
	Evidence     []agent.Evidence    `json:"evidence"`
	AllowedTools []string            `json:"allowed_tools"`
	Vars         map[string]any      `json:"vars"`
	Budgets      BudgetStatus        `json:"budgets"`
}

// BudgetStatus tracks remaining budget across different dimensions.
type BudgetStatus struct {
	Remaining map[string]int `json:"remaining"`
}

// Config configures the LLM planner.
type Config struct {
	// Provider is the LLM provider to use.
	Provider Provider

	// Model is the model identifier (provider-specific).
	Model string

	// Temperature controls randomness (0.0 to 1.0).
	Temperature float64

	// MaxTokens limits the response length.
	MaxTokens int

	// SystemPrompt overrides the default system prompt.
	SystemPrompt string

	// EnableStreaming enables streaming responses if the provider supports it.
	EnableStreaming bool
}

// LLMPlanner uses an LLM provider to make planning decisions.
type LLMPlanner struct {
	provider     Provider
	model        string
	temperature  float64
	maxTokens    int
	systemPrompt string
	streaming    bool
}

// NewPlanner creates a new LLM-based planner with the given configuration.
func NewPlanner(cfg Config) *LLMPlanner {
	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	return &LLMPlanner{
		provider:     cfg.Provider,
		model:        cfg.Model,
		temperature:  temperature,
		maxTokens:    maxTokens,
		systemPrompt: systemPrompt,
		streaming:    cfg.EnableStreaming,
	}
}

// Plan implements the Planner interface.
// It builds a prompt from the plan request, sends it to the LLM provider,
// and parses the response into a Decision.
func (p *LLMPlanner) Plan(ctx context.Context, req PlanRequest) (agent.Decision, error) {
	// TODO: Implement actual planning logic
	// 1. Build messages from plan request
	// 2. Send completion request to provider
	// 3. Parse response into agent.Decision

	return agent.Decision{
		Type: agent.DecisionFail,
		Fail: &agent.FailDecision{
			Reason: "LLM planner not yet implemented",
		},
	}, nil
}

// DefaultSystemPrompt is the default system prompt for the agent planner.
const DefaultSystemPrompt = `You are an AI agent that helps accomplish goals by making decisions and using tools.

Your role is to analyze the current state, evidence, and available tools to decide the next action.

## Response Format

You MUST respond with a JSON object in one of these formats:

### 1. Call a Tool
{"decision": "call_tool", "tool_name": "<name>", "input": {...}, "reason": "<why>"}

### 2. Transition State
{"decision": "transition", "to_state": "<state>", "reason": "<why>"}

Valid states: intake, explore, decide, act, validate, done, failed

### 3. Finish Successfully
{"decision": "finish", "result": <any>, "summary": "<brief summary>"}

### 4. Fail
{"decision": "fail", "reason": "<why failed>"}

## Guidelines

1. In "explore" state: Gather information using read-only tools
2. In "act" state: Execute actions using available tools
3. In "validate" state: Verify results and decide if goal is achieved
4. Always provide a reason for your decisions
5. Respond ONLY with valid JSON, no additional text`

// Ensure LLMPlanner implements Planner interface.
var _ Planner = (*LLMPlanner)(nil)
