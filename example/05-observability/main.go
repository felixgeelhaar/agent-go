// Package main demonstrates observability integration with OpenTelemetry.
// Shows tracing, metrics, and structured logging.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	agent "github.com/felixgeelhaar/agent-go/interfaces/api"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/observability"
)

func main() {
	fmt.Println("=== Observability Example ===")
	fmt.Println()

	// ============================================
	// Set up OpenTelemetry tracing and metrics
	// ============================================

	// Create tracer (uses global OpenTelemetry provider)
	// In production, configure OTLP exporter before this
	tracer := observability.NewOTelTracer("calculator-agent")

	// Create meter for metrics
	meter := observability.NewOTelMeter("calculator-agent")

	fmt.Println("Tracer and meter initialized")
	fmt.Println()

	// ============================================
	// Create tools with simulated latency
	// ============================================

	slowTool := agent.NewToolBuilder("slow_operation").
		WithDescription("A slow operation that takes time").
		WithAnnotations(agent.Annotations{ReadOnly: true}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			// Simulate work
			time.Sleep(100 * time.Millisecond)

			output, _ := json.Marshal(map[string]string{"status": "completed"})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	fastTool := agent.NewToolBuilder("fast_operation").
		WithDescription("A fast operation").
		WithAnnotations(agent.Annotations{ReadOnly: true}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			// Quick operation
			time.Sleep(10 * time.Millisecond)

			output, _ := json.Marshal(map[string]string{"status": "done"})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Build engine with observability middleware
	// ============================================

	planner := agent.NewScriptedPlanner(
		agent.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("fast_operation", nil, "quick check"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("slow_operation", nil, "detailed analysis"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("fast_operation", nil, "final check"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "analysis complete"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("completed", json.RawMessage(`{"result":"success"}`)),
		},
	)

	eligibility := agent.NewToolEligibility()
	eligibility.AllowMultiple(agent.StateExplore, "slow_operation", "fast_operation")

	engine, err := agent.New(
		agent.WithTool(slowTool),
		agent.WithTool(fastTool),
		agent.WithPlanner(planner),
		agent.WithToolEligibility(eligibility),
		// Add observability middleware
		agent.WithMiddleware(
			observability.TracingMiddleware(tracer),
			observability.MetricsMiddleware(meter),
			agent.LoggingMiddleware(nil),
		),
		agent.WithMaxSteps(10),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ============================================
	// Run the agent
	// ============================================

	fmt.Println("Running agent with tracing enabled...")
	fmt.Println()

	ctx := context.Background()
	run, err := engine.Run(ctx, "Perform analysis with observability")
	if err != nil {
		log.Fatal(err)
	}

	// ============================================
	// Display results
	// ============================================

	fmt.Println()
	fmt.Println("=== Run Complete ===")
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Steps: %d\n", len(run.Evidence))
	fmt.Printf("Duration: %s\n", run.Duration())

	// ============================================
	// Show what was captured
	// ============================================

	fmt.Println()
	fmt.Println("=== What Was Captured ===")
	fmt.Println()
	fmt.Println("Traces (viewable in Jaeger/Honeycomb/etc.):")
	fmt.Println("  - agent.run (root span)")
	fmt.Println("    - tool.execute: fast_operation (10ms)")
	fmt.Println("    - tool.execute: slow_operation (100ms)")
	fmt.Println("    - tool.execute: fast_operation (10ms)")
	fmt.Println()
	fmt.Println("Metrics:")
	fmt.Println("  - tool_executions_total{tool=fast_operation, status=success} = 2")
	fmt.Println("  - tool_executions_total{tool=slow_operation, status=success} = 1")
	fmt.Println("  - tool_duration_seconds{tool=fast_operation} histogram")
	fmt.Println("  - tool_duration_seconds{tool=slow_operation} histogram")
	fmt.Println("  - runs_total{status=done} = 1")
	fmt.Println()
	fmt.Println("Logs (structured JSON):")
	fmt.Println(`  {"level":"info","run_id":"...","state":"explore","tool":"fast_operation","duration_ms":10}`)
	fmt.Println(`  {"level":"info","run_id":"...","state":"explore","tool":"slow_operation","duration_ms":100}`)
}
