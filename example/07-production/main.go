// Package main demonstrates a production-ready agent setup.
// Combines all features: LLM planning, observability, security, and error handling.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	agent "github.com/felixgeelhaar/agent-go/interfaces/api"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/observability"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/audit"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/validation"
)

func main() {
	fmt.Println("=== Production Agent Example ===")
	fmt.Println()

	// ============================================
	// Configuration from environment
	// ============================================

	config := loadConfig()

	fmt.Printf("Environment: %s\n", config.Environment)
	fmt.Printf("Service: %s v%s\n", config.ServiceName, config.ServiceVersion)
	fmt.Println()

	// ============================================
	// Set up observability
	// ============================================

	tracer := observability.NewOTelTracer(config.ServiceName)
	meter := observability.NewOTelMeter(config.ServiceName)

	fmt.Println("Observability initialized")

	// ============================================
	// Set up security
	// ============================================

	// Input validation schema for file paths
	pathSchema := validation.NewSchema().
		AddRule("path", validation.Required()).
		AddRule("path", validation.MaxLength(1000)).
		AddRule("path", validation.NoPathTraversal())

	validationSchemas := map[string]*validation.Schema{
		"read_file": pathSchema,
	}

	// Audit logger
	auditLogger := audit.NewJSONLogger(os.Stdout)

	fmt.Println("Security initialized")

	// ============================================
	// Create tools with validation
	// ============================================

	registry := agent.NewToolRegistry()

	// Validated file reading tool
	readFile := agent.NewToolBuilder("read_file").
		WithDescription("Reads file contents with path validation").
		WithAnnotations(agent.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  agent.RiskLow,
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate input
			if err := pathSchema.Validate(input); err != nil {
				_ = auditLogger.Log(ctx, audit.Event{
					EventType:  audit.EventPolicyViolation,
					ToolName:   "read_file",
					Success:    false,
					Error:      err.Error(),
					Annotations: map[string]interface{}{
						"path":    in.Path,
						"service": config.ServiceName,
						"env":     config.Environment,
					},
				}) // Ignore audit log error, primary error returned below
				return tool.Result{}, fmt.Errorf("invalid path: %w", err)
			}

			content, err := os.ReadFile(in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			_ = auditLogger.Log(ctx, audit.Event{
				EventType: audit.EventToolExecution,
				ToolName:  "read_file",
				Success:   true,
				Annotations: map[string]interface{}{
					"path":    in.Path,
					"size":    len(content),
					"service": config.ServiceName,
					"env":     config.Environment,
				},
			}) // Ignore audit log error in example

			output, err := json.Marshal(map[string]any{
				"content": string(content),
				"size":    len(content),
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal output: %w", err)
			}
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	_ = registry.Register(readFile) // Ignore error in example

	fmt.Println("Tools registered")

	// ============================================
	// Set up planner
	// ============================================

	var agentPlanner planner.Planner

	if config.AnthropicKey != "" {
		provider := planner.NewAnthropicProvider(planner.AnthropicConfig{
			APIKey:  config.AnthropicKey,
			Model:   "claude-sonnet-4-20250514",
			Timeout: 30,
		})

		agentPlanner = planner.NewLLMPlanner(planner.LLMPlannerConfig{
			Provider:    provider,
			Temperature: 0.3,
			SystemPrompt: `You are a helpful file assistant. You can read files to answer questions.
Always validate that files exist before reading.
Be concise in your responses.`,
		})
		fmt.Println("LLM planner initialized (Anthropic)")
	} else {
		// Fallback to scripted planner for demo
		agentPlanner = agent.NewScriptedPlanner(
			agent.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
			},
			agent.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewTransitionDecision(agent.StateDecide, "no API key, finishing"),
			},
			agent.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    agent.NewFinishDecision("No API key configured", nil),
			},
		)
		fmt.Println("Using scripted planner (set ANTHROPIC_API_KEY for LLM)")
	}

	// ============================================
	// Set up policies
	// ============================================

	eligibility := agent.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")
	eligibility.Allow(agent.StateValidate, "read_file")

	// Auto-approver for this demo (use interactive approver in production)
	approver := agent.AutoApprover()

	fmt.Println("Policies configured")

	// ============================================
	// Build engine
	// ============================================

	engine, err := agent.New(
		agent.WithRegistry(registry),
		agent.WithPlanner(agentPlanner),
		agent.WithToolEligibility(eligibility),
		agent.WithApprover(approver),

		// Budgets
		agent.WithBudget("tool_calls", 20),
		agent.WithBudget("tokens", 10000),
		agent.WithMaxSteps(30),

		// Middleware
		agent.WithMiddleware(
			observability.TracingMiddleware(tracer),
			observability.MetricsMiddleware(meter),
			validation.ValidationMiddleware(validationSchemas),
			audit.AuditMiddleware(auditLogger),
			agent.LoggingMiddleware(nil),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	fmt.Println("Engine created")
	fmt.Println()

	// ============================================
	// Graceful shutdown setup
	// ============================================

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal...")
		cancel()
	}()

	// ============================================
	// Run the agent
	// ============================================

	fmt.Println("Running agent...")
	fmt.Println()

	goal := "What files are in the current directory?"
	run, err := engine.Run(ctx, goal)
	if err != nil {
		cancel() // Explicit cleanup before exit
		if ctx.Err() != nil {
			fmt.Println("Agent interrupted by shutdown signal")
			return
		}
		log.Fatalf("Agent execution failed: %v", err)
	}
	defer cancel() // Cleanup on normal exit path

	// ============================================
	// Display results
	// ============================================

	fmt.Println()
	fmt.Println("=== Run Complete ===")
	fmt.Printf("Run ID: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Steps: %d\n", len(run.Evidence))
	fmt.Printf("Duration: %s\n", run.Duration())

	if run.Result != nil {
		fmt.Printf("Result: %s\n", string(run.Result))
	}

	if run.Status == agent.StatusFailed {
		fmt.Printf("Failure: %s\n", run.Error)
	}

	// Log completion to audit trail
	_ = auditLogger.Log(ctx, audit.Event{
		EventType: audit.EventRunComplete,
		RunID:     run.ID,
		Success:   run.Status == agent.StatusCompleted,
		Annotations: map[string]interface{}{
			"status":   string(run.Status),
			"steps":    len(run.Evidence),
			"duration": run.Duration().String(),
		},
	}) // Ignore audit log error in example
}

// Config holds application configuration
type Config struct {
	Environment    string
	ServiceName    string
	ServiceVersion string
	AnthropicKey   string
	OTLPEndpoint   string
}

func loadConfig() Config {
	return Config{
		Environment:    getEnv("ENVIRONMENT", "development"),
		ServiceName:    getEnv("SERVICE_NAME", "file-agent"),
		ServiceVersion: getEnv("SERVICE_VERSION", "1.0.0"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
