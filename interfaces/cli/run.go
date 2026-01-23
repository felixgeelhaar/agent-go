package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// runOptions holds options for the run command.
type runOptions struct {
	configPath  string
	goal        string
	maxSteps    int
	timeout     time.Duration
	verbose     bool
	jsonOutput  bool
	dryRun      bool
	initialVars map[string]string
}

// newRunCmd creates the run command.
func (a *App) newRunCmd() *cobra.Command {
	opts := &runOptions{
		initialVars: make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:   "run [goal]",
		Short: "Run an agent with the specified goal",
		Long: `Run an agent using the provided configuration file and goal.

The agent will execute according to its state machine, using the configured
tools and policies until it reaches a terminal state (done or failed).

Examples:
  # Run with a config file and goal as argument
  agent run -c config.yaml "Process the data files"

  # Run with goal from stdin
  echo "Analyze the log files" | agent run -c config.yaml

  # Run with custom timeout and max steps
  agent run -c config.yaml --timeout 5m --max-steps 50 "Run analysis"

  # Run with variables
  agent run -c config.yaml --var env=production --var debug=true "Deploy"

  # Dry run (validate configuration without executing)
  agent run -c config.yaml --dry-run "Test goal"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.goal = args[0]
			}
			return a.runAgent(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configPath, "config", "c", "", "Path to configuration file (required)")
	cmd.Flags().IntVar(&opts.maxSteps, "max-steps", 0, "Maximum execution steps (overrides config)")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 0, "Execution timeout (overrides config)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Validate configuration without executing")
	cmd.Flags().StringToStringVar(&opts.initialVars, "var", nil, "Set variables (key=value)")

	_ = cmd.MarkFlagRequired("config")

	return cmd
}

// runAgent executes the agent with the given options.
func (a *App) runAgent(ctx context.Context, opts *runOptions) error {
	// Load configuration
	loader := api.NewConfigLoader()
	config, err := loader.LoadFile(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config values with CLI options
	if opts.maxSteps > 0 {
		config.Agent.MaxSteps = opts.maxSteps
	}

	// Merge CLI variables into config variables
	if config.Variables == nil {
		config.Variables = make(map[string]any)
	}
	for k, v := range opts.initialVars {
		config.Variables[k] = v
	}

	// Build engine components from config
	builder := api.NewConfigBuilder(config)
	result, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to build engine configuration: %w", err)
	}

	if opts.verbose {
		_, _ = fmt.Fprintf(a.stdout, "Configuration loaded: %s v%s\n", config.Name, config.Version)
		if config.Description != "" {
			_, _ = fmt.Fprintf(a.stdout, "Description: %s\n", config.Description)
		}
		_, _ = fmt.Fprintf(a.stdout, "Max steps: %d\n", result.MaxSteps)
		_, _ = fmt.Fprintf(a.stdout, "Initial state: %s\n", config.Agent.InitialState)
		_, _ = fmt.Fprintf(a.stdout, "Tool packs: %d\n", len(result.ToolPacks))
		_, _ = fmt.Fprintf(a.stdout, "Inline tools: %d\n", len(result.InlineTools))
		_, _ = fmt.Fprintf(a.stdout, "\n")
	}

	// If dry-run, stop here
	if opts.dryRun {
		_, _ = fmt.Fprintf(a.stdout, "Configuration validated successfully.\n")
		if opts.goal != "" {
			_, _ = fmt.Fprintf(a.stdout, "Goal: %s\n", opts.goal)
		}
		return nil
	}

	// Check for goal
	goal := opts.goal
	if goal == "" {
		goal = config.Agent.DefaultGoal
	}
	if goal == "" {
		return fmt.Errorf("no goal specified (use argument or set agent.default_goal in config)")
	}

	// Create a scripted planner that properly traverses the state machine
	// In a real implementation, this would come from the configuration (e.g., LLM planner)
	planner := api.NewScriptedPlanner(
		api.ScriptStep{
			ExpectState: api.StateIntake,
			Decision:    api.NewTransitionDecision(api.StateExplore, "begin exploration"),
		},
		api.ScriptStep{
			ExpectState: api.StateExplore,
			Decision:    api.NewTransitionDecision(api.StateDecide, "ready to decide"),
		},
		api.ScriptStep{
			ExpectState: api.StateDecide,
			Decision:    api.NewFinishDecision("completed", json.RawMessage(`{"status": "success"}`)),
		},
	)

	// Build engine options
	engineOpts := []api.Option{
		api.WithPlanner(planner),
		api.WithMaxSteps(result.MaxSteps),
		api.WithBudgets(result.Budgets),
	}

	if result.Eligibility != nil {
		engineOpts = append(engineOpts, api.WithToolEligibility(result.Eligibility))
	}

	if result.RateLimitConfig != nil && result.RateLimitConfig.Enabled {
		engineOpts = append(engineOpts, api.WithRateLimit(
			result.RateLimitConfig.Rate,
			result.RateLimitConfig.Burst,
		))
	}

	// Create engine
	engine, err := api.New(engineOpts...)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// Apply timeout if specified
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	if opts.verbose {
		_, _ = fmt.Fprintf(a.stdout, "Starting agent run...\n")
		_, _ = fmt.Fprintf(a.stdout, "Goal: %s\n", goal)
		_, _ = fmt.Fprintf(a.stdout, "\n")
	}

	// Execute the run with variables
	startTime := time.Now()
	run, err := engine.RunWithVars(ctx, goal, result.Variables)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("agent run failed: %w", err)
	}

	// Output results
	if opts.jsonOutput {
		output := map[string]any{
			"run_id":   run.ID,
			"state":    run.CurrentState.String(),
			"goal":     goal,
			"duration": duration.String(),
		}

		if run.Result != nil {
			output["result"] = json.RawMessage(run.Result)
		}
		if run.Error != "" {
			output["error"] = run.Error
		}

		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Text output
	_, _ = fmt.Fprintf(a.stdout, "Run completed\n")
	_, _ = fmt.Fprintf(a.stdout, "  Run ID: %s\n", run.ID)
	_, _ = fmt.Fprintf(a.stdout, "  State: %s\n", run.CurrentState.String())
	_, _ = fmt.Fprintf(a.stdout, "  Duration: %s\n", duration)

	switch run.CurrentState {
	case agent.StateDone:
		_, _ = fmt.Fprintf(a.stdout, "  Status: SUCCESS\n")
		if run.Result != nil {
			_, _ = fmt.Fprintf(a.stdout, "  Result: %s\n", formatJSON(run.Result))
		}
	case agent.StateFailed:
		_, _ = fmt.Fprintf(a.stdout, "  Status: FAILED\n")
		if run.Error != "" {
			_, _ = fmt.Fprintf(a.stdout, "  Error: %s\n", run.Error)
		}
	}

	return nil
}

// formatJSON formats JSON for display.
func formatJSON(data json.RawMessage) string {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	formatted, err := json.MarshalIndent(v, "          ", "  ")
	if err != nil {
		return string(data)
	}
	// Replace newlines to align with prefix
	return strings.TrimSpace(string(formatted))
}
