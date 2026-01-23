package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// inspectOptions holds options for the inspect command.
type inspectOptions struct {
	configPath string
	outputJSON bool
	section    string
}

// newInspectCmd creates the inspect command.
func (a *App) newInspectCmd() *cobra.Command {
	opts := &inspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect configuration details",
		Long: `Inspect and display detailed configuration information.

This command provides a comprehensive view of the agent configuration,
including all settings, tools, policies, and resilience configuration.

Sections:
  all         Show all configuration (default)
  agent       Show agent settings
  tools       Show tools configuration
  policy      Show policy settings
  resilience  Show resilience configuration
  notification Show notification configuration

Examples:
  # Inspect full configuration
  agent inspect -c config.yaml

  # Inspect specific section
  agent inspect -c config.yaml --section tools

  # Output as JSON
  agent inspect -c config.yaml --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.inspectConfig(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configPath, "config", "c", "", "Path to configuration file (required)")
	cmd.Flags().BoolVar(&opts.outputJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&opts.section, "section", "all", "Section to inspect (all, agent, tools, policy, resilience, notification)")

	_ = cmd.MarkFlagRequired("config")

	return cmd
}

// inspectConfig inspects the configuration.
func (a *App) inspectConfig(opts *inspectOptions) error {
	loader := api.NewConfigLoader()
	config, err := loader.LoadFile(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if opts.outputJSON {
		return a.inspectJSON(config, opts.section)
	}

	return a.inspectText(config, opts.section)
}

// inspectJSON outputs configuration as JSON.
func (a *App) inspectJSON(config *api.AgentConfig, section string) error {
	var output any

	switch section {
	case "all":
		output = config
	case "agent":
		output = config.Agent
	case "tools":
		output = config.Tools
	case "policy":
		output = config.Policy
	case "resilience":
		output = config.Resilience
	case "notification":
		output = config.Notification
	default:
		return fmt.Errorf("unknown section: %s", section)
	}

	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// inspectText outputs configuration as formatted text.
func (a *App) inspectText(config *api.AgentConfig, section string) error {
	switch section {
	case "all":
		a.printHeader(config)
		a.printAgentSection(config)
		a.printToolsSection(config)
		a.printPolicySection(config)
		a.printResilienceSection(config)
		a.printNotificationSection(config)
		a.printVariablesSection(config)
	case "agent":
		a.printAgentSection(config)
	case "tools":
		a.printToolsSection(config)
	case "policy":
		a.printPolicySection(config)
	case "resilience":
		a.printResilienceSection(config)
	case "notification":
		a.printNotificationSection(config)
	default:
		return fmt.Errorf("unknown section: %s", section)
	}

	return nil
}

func (a *App) printHeader(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Agent Configuration: %s\n", config.Name)
	_, _ = fmt.Fprintf(a.stdout, "═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(a.stdout, "Version: %s\n", config.Version)
	if config.Description != "" {
		_, _ = fmt.Fprintf(a.stdout, "Description: %s\n", config.Description)
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printAgentSection(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Agent Settings\n")
	_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")
	_, _ = fmt.Fprintf(a.stdout, "  Max Steps: %d\n", config.Agent.MaxSteps)
	if config.Agent.InitialState != "" {
		_, _ = fmt.Fprintf(a.stdout, "  Initial State: %s\n", config.Agent.InitialState)
	}
	if config.Agent.DefaultGoal != "" {
		_, _ = fmt.Fprintf(a.stdout, "  Default Goal: %s\n", config.Agent.DefaultGoal)
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printToolsSection(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Tools Configuration\n")
	_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")

	if len(config.Tools.Packs) == 0 && len(config.Tools.Inline) == 0 {
		_, _ = fmt.Fprintf(a.stdout, "  No tools configured\n")
	} else {
		if len(config.Tools.Packs) > 0 {
			_, _ = fmt.Fprintf(a.stdout, "  Packs (%d):\n", len(config.Tools.Packs))
			for _, pack := range config.Tools.Packs {
				_, _ = fmt.Fprintf(a.stdout, "    • %s", pack.Name)
				if pack.Version != "" {
					_, _ = fmt.Fprintf(a.stdout, " v%s", pack.Version)
				}
				_, _ = fmt.Fprintln(a.stdout)
			}
		}

		if len(config.Tools.Inline) > 0 {
			_, _ = fmt.Fprintf(a.stdout, "  Inline Tools (%d):\n", len(config.Tools.Inline))
			for _, tool := range config.Tools.Inline {
				_, _ = fmt.Fprintf(a.stdout, "    • %s", tool.Name)
				if tool.Description != "" {
					_, _ = fmt.Fprintf(a.stdout, " - %s", tool.Description)
				}
				_, _ = fmt.Fprintln(a.stdout)
			}
		}

		if len(config.Tools.Eligibility) > 0 {
			_, _ = fmt.Fprintf(a.stdout, "  Eligibility:\n")
			for state, tools := range config.Tools.Eligibility {
				_, _ = fmt.Fprintf(a.stdout, "    %s: %d tools\n", state, len(tools))
			}
		}
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printPolicySection(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Policy Configuration\n")
	_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")

	if len(config.Policy.Budgets) > 0 {
		_, _ = fmt.Fprintf(a.stdout, "  Budgets:\n")
		for name, limit := range config.Policy.Budgets {
			_, _ = fmt.Fprintf(a.stdout, "    • %s: %d\n", name, limit)
		}
	}

	_, _ = fmt.Fprintf(a.stdout, "  Approval:\n")
	_, _ = fmt.Fprintf(a.stdout, "    Mode: %s\n", config.Policy.Approval.Mode)
	_, _ = fmt.Fprintf(a.stdout, "    Require for Destructive: %v\n", config.Policy.Approval.RequireForDestructive)

	if config.Policy.RateLimit.Enabled {
		_, _ = fmt.Fprintf(a.stdout, "  Rate Limiting:\n")
		_, _ = fmt.Fprintf(a.stdout, "    Enabled: true\n")
		_, _ = fmt.Fprintf(a.stdout, "    Rate: %d/s\n", config.Policy.RateLimit.Rate)
		_, _ = fmt.Fprintf(a.stdout, "    Burst: %d\n", config.Policy.RateLimit.Burst)
		if config.Policy.RateLimit.PerTool {
			_, _ = fmt.Fprintf(a.stdout, "    Per-Tool: true\n")
		}
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printResilienceSection(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Resilience Configuration\n")
	_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")

	if config.Resilience.Timeout.Duration() > 0 {
		_, _ = fmt.Fprintf(a.stdout, "  Timeout: %s\n", config.Resilience.Timeout.Duration())
	}

	if config.Resilience.Retry.Enabled {
		_, _ = fmt.Fprintf(a.stdout, "  Retry:\n")
		_, _ = fmt.Fprintf(a.stdout, "    Max Attempts: %d\n", config.Resilience.Retry.MaxAttempts)
		_, _ = fmt.Fprintf(a.stdout, "    Initial Delay: %s\n", config.Resilience.Retry.InitialDelay.Duration())
		_, _ = fmt.Fprintf(a.stdout, "    Multiplier: %.1f\n", config.Resilience.Retry.Multiplier)
	}

	if config.Resilience.CircuitBreaker.Enabled {
		_, _ = fmt.Fprintf(a.stdout, "  Circuit Breaker:\n")
		_, _ = fmt.Fprintf(a.stdout, "    Threshold: %d failures\n", config.Resilience.CircuitBreaker.Threshold)
		_, _ = fmt.Fprintf(a.stdout, "    Timeout: %s\n", config.Resilience.CircuitBreaker.Timeout.Duration())
	}

	if config.Resilience.Bulkhead.Enabled {
		_, _ = fmt.Fprintf(a.stdout, "  Bulkhead:\n")
		_, _ = fmt.Fprintf(a.stdout, "    Max Concurrent: %d\n", config.Resilience.Bulkhead.MaxConcurrent)
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printNotificationSection(config *api.AgentConfig) {
	_, _ = fmt.Fprintf(a.stdout, "Notification Configuration\n")
	_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")

	if !config.Notification.Enabled {
		_, _ = fmt.Fprintf(a.stdout, "  Enabled: false\n")
	} else {
		_, _ = fmt.Fprintf(a.stdout, "  Enabled: true\n")
		if len(config.Notification.Endpoints) > 0 {
			_, _ = fmt.Fprintf(a.stdout, "  Endpoints (%d):\n", len(config.Notification.Endpoints))
			for _, ep := range config.Notification.Endpoints {
				_, _ = fmt.Fprintf(a.stdout, "    • %s", ep.Name)
				if !ep.Enabled {
					_, _ = fmt.Fprintf(a.stdout, " (disabled)")
				}
				_, _ = fmt.Fprintln(a.stdout)
			}
		}
		if config.Notification.Batching.Enabled {
			_, _ = fmt.Fprintf(a.stdout, "  Batching:\n")
			_, _ = fmt.Fprintf(a.stdout, "    Max Size: %d\n", config.Notification.Batching.MaxSize)
			_, _ = fmt.Fprintf(a.stdout, "    Max Wait: %s\n", config.Notification.Batching.MaxWait.Duration())
		}
	}
	_, _ = fmt.Fprintln(a.stdout)
}

func (a *App) printVariablesSection(config *api.AgentConfig) {
	if len(config.Variables) > 0 {
		_, _ = fmt.Fprintf(a.stdout, "Variables\n")
		_, _ = fmt.Fprintf(a.stdout, "───────────────────────────────────────\n")
		for k, v := range config.Variables {
			_, _ = fmt.Fprintf(a.stdout, "  %s: %v\n", k, v)
		}
		_, _ = fmt.Fprintln(a.stdout)
	}
}
