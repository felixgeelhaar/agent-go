package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/infrastructure/logging"
)

// LLMPlanner uses an LLM provider to make planning decisions.
type LLMPlanner struct {
	provider    Provider
	model       string
	temperature float64
	maxTokens   int
	systemPrompt string
}

// LLMPlannerConfig configures the LLM planner.
type LLMPlannerConfig struct {
	Provider     Provider
	Model        string
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
}

// DefaultSystemPrompt is the default system prompt for the agent.
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

// NewLLMPlanner creates a new LLM-based planner.
func NewLLMPlanner(config LLMPlannerConfig) *LLMPlanner {
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	temperature := config.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	return &LLMPlanner{
		provider:     config.Provider,
		model:        config.Model,
		temperature:  temperature,
		maxTokens:    maxTokens,
		systemPrompt: systemPrompt,
	}
}

// Plan implements the Planner interface.
func (p *LLMPlanner) Plan(ctx context.Context, req PlanRequest) (agent.Decision, error) {
	// Build messages
	messages := p.buildMessages(req)

	// Make completion request
	completionReq := CompletionRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
	}

	logging.Debug().
		Add(logging.RunID(req.RunID)).
		Add(logging.State(req.CurrentState)).
		Msg("requesting LLM decision")

	resp, err := p.provider.Complete(ctx, completionReq)
	if err != nil {
		return agent.Decision{}, fmt.Errorf("LLM completion failed: %w", err)
	}

	if resp.Error != nil {
		return agent.Decision{}, resp.Error
	}

	// Parse the response
	decision, err := p.parseResponse(resp.Message.Content)
	if err != nil {
		return agent.Decision{}, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	logging.Debug().
		Add(logging.RunID(req.RunID)).
		Add(logging.Decision(decision.Type)).
		Msg("LLM decision received")

	return decision, nil
}

// buildMessages constructs the message history for the LLM.
func (p *LLMPlanner) buildMessages(req PlanRequest) []Message {
	messages := []Message{
		{Role: "system", Content: p.systemPrompt},
	}

	// Build user message with current context
	var sb strings.Builder

	sb.WriteString("## Current State\n")
	sb.WriteString(fmt.Sprintf("State: %s\n", req.CurrentState))
	sb.WriteString(fmt.Sprintf("Run ID: %s\n\n", req.RunID))

	if len(req.AllowedTools) > 0 {
		sb.WriteString("## Available Tools\n")
		for _, tool := range req.AllowedTools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	if len(req.Budgets.Remaining) > 0 {
		sb.WriteString("## Budgets\n")
		for name, remaining := range req.Budgets.Remaining {
			sb.WriteString(fmt.Sprintf("- %s: %d remaining\n", name, remaining))
		}
		sb.WriteString("\n")
	}

	if len(req.Evidence) > 0 {
		sb.WriteString("## Evidence\n")
		for i, ev := range req.Evidence {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, ev.Type, ev.Source))
			// Truncate long evidence
			content := string(ev.Content)
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", content))
		}
		sb.WriteString("\n")
	}

	if len(req.Vars) > 0 {
		sb.WriteString("## Variables\n")
		for k, v := range req.Vars {
			sb.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("What is your next decision? Respond with JSON only.")

	messages = append(messages, Message{
		Role:    "user",
		Content: sb.String(),
	})

	return messages
}

// llmResponse represents the expected JSON response from the LLM.
type llmResponse struct {
	Decision string          `json:"decision"`
	ToolName string          `json:"tool_name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	ToState  string          `json:"to_state,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
	Summary  string          `json:"summary,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

// parseResponse parses the LLM response into a Decision.
func (p *LLMPlanner) parseResponse(content string) (agent.Decision, error) {
	// Clean up the response - remove markdown code blocks if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var resp llmResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return agent.Decision{}, fmt.Errorf("invalid JSON response: %w (content: %s)", err, truncate(content, 200))
	}

	switch resp.Decision {
	case "call_tool":
		return agent.Decision{
			Type: agent.DecisionCallTool,
			CallTool: &agent.CallToolDecision{
				ToolName: resp.ToolName,
				Input:    resp.Input,
				Reason:   resp.Reason,
			},
		}, nil

	case "transition":
		return agent.Decision{
			Type: agent.DecisionTransition,
			Transition: &agent.TransitionDecision{
				ToState: agent.State(resp.ToState),
				Reason:  resp.Reason,
			},
		}, nil

	case "finish":
		result := resp.Result
		if result == nil {
			result = json.RawMessage(`null`)
		}
		return agent.Decision{
			Type: agent.DecisionFinish,
			Finish: &agent.FinishDecision{
				Result:  result,
				Summary: resp.Summary,
			},
		}, nil

	case "fail":
		return agent.Decision{
			Type: agent.DecisionFail,
			Fail: &agent.FailDecision{
				Reason: resp.Reason,
			},
		}, nil

	default:
		return agent.Decision{}, fmt.Errorf("unknown decision type: %s", resp.Decision)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
