// Package main demonstrates using a real LLM provider for planning.
// Shows how to swap from scripted to intelligent planning.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	agent "github.com/felixgeelhaar/agent-go/interfaces/api"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
)

func main() {
	// Check for API keys
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	ollamaURL := os.Getenv("OLLAMA_URL") // e.g., http://localhost:11434

	// ============================================
	// Create tools for the agent
	// ============================================

	calculateTool := agent.NewToolBuilder("calculate").
		WithDescription("Performs basic arithmetic. Supports add, subtract, multiply, divide operations.").
		WithAnnotations(agent.Annotations{
			ReadOnly:   true,
			Idempotent: true,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"operation": {"type": "string", "enum": ["add", "subtract", "multiply", "divide"]},
				"a": {"type": "number"},
				"b": {"type": "number"}
			},
			"required": ["operation", "a", "b"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Operation string  `json:"operation"`
				A         float64 `json:"a"`
				B         float64 `json:"b"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			var result float64
			switch in.Operation {
			case "add":
				result = in.A + in.B
			case "subtract":
				result = in.A - in.B
			case "multiply":
				result = in.A * in.B
			case "divide":
				if in.B == 0 {
					return tool.Result{}, fmt.Errorf("division by zero")
				}
				result = in.A / in.B
			default:
				return tool.Result{}, fmt.Errorf("unknown operation: %s", in.Operation)
			}

			fmt.Printf("  [calculate] %s(%g, %g) = %g\n", in.Operation, in.A, in.B, result)

			output, _ := json.Marshal(map[string]float64{"result": result})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Select LLM provider based on available credentials
	// ============================================

	var llmPlanner planner.Planner
	var providerName string

	switch {
	case anthropicKey != "":
		providerName = "Anthropic (Claude)"
		provider := planner.NewAnthropicProvider(planner.AnthropicConfig{
			APIKey: anthropicKey,
			Model:  "claude-sonnet-4-20250514",
		})
		llmPlanner = planner.NewLLMPlanner(planner.LLMPlannerConfig{
			Provider:    provider,
			Temperature: 0.3,
			SystemPrompt: `You are a helpful calculator agent. You can perform arithmetic operations.
When given a math problem, use the calculate tool to solve it step by step.
After getting the result, finish with the final answer.`,
		})

	case openaiKey != "":
		providerName = "OpenAI (GPT-4)"
		provider := planner.NewOpenAIProvider(planner.OpenAIConfig{
			APIKey: openaiKey,
			Model:  "gpt-4-turbo",
		})
		llmPlanner = planner.NewLLMPlanner(planner.LLMPlannerConfig{
			Provider:    provider,
			Temperature: 0.3,
			SystemPrompt: `You are a helpful calculator agent. Use the calculate tool for arithmetic.`,
		})

	case ollamaURL != "":
		providerName = "Ollama (Local)"
		provider := planner.NewOllamaProvider(planner.OllamaConfig{
			BaseURL: ollamaURL,
			Model:   "llama3",
		})
		llmPlanner = planner.NewLLMPlanner(planner.LLMPlannerConfig{
			Provider:    provider,
			Temperature: 0.3,
		})

	default:
		fmt.Println("=== LLM Planner Example ===")
		fmt.Println()
		fmt.Println("No LLM provider configured. Set one of:")
		fmt.Println("  - ANTHROPIC_API_KEY for Claude")
		fmt.Println("  - OPENAI_API_KEY for GPT-4")
		fmt.Println("  - OLLAMA_URL for local Ollama (e.g., http://localhost:11434)")
		fmt.Println()
		fmt.Println("Running with MockPlanner instead...")
		fmt.Println()

		// Fall back to mock planner for demonstration
		providerName = "Mock (Demonstration)"
		llmPlanner = agent.NewScriptedPlanner(
			agent.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting calculation"),
			},
			agent.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision: agent.NewCallToolDecision("calculate",
					json.RawMessage(`{"operation":"multiply","a":15,"b":7}`),
					"multiplying 15 by 7"),
			},
			agent.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewTransitionDecision(agent.StateDecide, "calculation complete"),
			},
			agent.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision: agent.NewFinishDecision("The result of 15 × 7 = 105",
					json.RawMessage(`{"answer":105}`)),
			},
		)
	}

	// ============================================
	// Build and run the engine
	// ============================================

	fmt.Println("=== LLM Planner Example ===")
	fmt.Printf("Provider: %s\n", providerName)
	fmt.Println()

	eligibility := agent.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "calculate")
	eligibility.Allow(agent.StateAct, "calculate")

	engine, err := agent.New(
		agent.WithTool(calculateTool),
		agent.WithPlanner(llmPlanner),
		agent.WithToolEligibility(eligibility),
		agent.WithBudget("tool_calls", 10),
		agent.WithMaxSteps(20),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Run with a math problem
	goal := "What is 15 multiplied by 7?"
	fmt.Printf("Goal: %s\n", goal)
	fmt.Println()

	run, err := engine.Run(context.Background(), goal)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("=== Result ===")
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Steps: %d\n", len(run.Evidence))
	if run.Result != nil {
		fmt.Printf("Result: %s\n", string(run.Result))
	}
	if run.Status == agent.StatusFailed {
		fmt.Printf("Failure: %s\n", run.Error)
	}
}
