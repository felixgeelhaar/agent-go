package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// validateOptions holds options for the validate command.
type validateOptions struct {
	configPath string
	strict     bool
	showSchema bool
}

// newValidateCmd creates the validate command.
func (a *App) newValidateCmd() *cobra.Command {
	opts := &validateOptions{}

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a configuration file",
		Long: `Validate an agent configuration file for correctness.

This command checks:
  - File format (YAML or JSON)
  - Required fields (name, version)
  - Field types and constraints
  - State references in eligibility and transitions
  - Environment variable references (in strict mode)

Examples:
  # Validate a configuration file
  agent validate -c config.yaml

  # Strict validation (fail on missing env vars)
  agent validate -c config.yaml --strict

  # Show the JSON schema for configuration
  agent validate --schema`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.showSchema {
				return a.showConfigSchema()
			}
			return a.validateConfig(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configPath, "config", "c", "", "Path to configuration file")
	cmd.Flags().BoolVar(&opts.strict, "strict", false, "Enable strict validation (fail on missing env vars)")
	cmd.Flags().BoolVar(&opts.showSchema, "schema", false, "Show JSON schema for configuration")

	return cmd
}

// validateConfig validates the configuration file.
func (a *App) validateConfig(opts *validateOptions) error {
	if opts.configPath == "" {
		return fmt.Errorf("configuration file path is required (-c flag)")
	}

	// Create loader with appropriate options
	loaderOpts := []api.ConfigLoaderOption{
		api.ConfigWithValidation(true),
	}
	if opts.strict {
		loaderOpts = append(loaderOpts, api.ConfigWithStrictEnv(true))
	}

	loader := api.NewConfigLoaderWithOptions(loaderOpts...)
	config, err := loader.LoadFile(opts.configPath)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Additional validation via the builder
	builder := api.NewConfigBuilder(config)
	_, err = builder.Build()
	if err != nil {
		return fmt.Errorf("configuration build failed: %w", err)
	}

	fmt.Fprintf(a.stdout, "✓ Configuration is valid\n")
	fmt.Fprintf(a.stdout, "  Name: %s\n", config.Name)
	fmt.Fprintf(a.stdout, "  Version: %s\n", config.Version)
	if config.Description != "" {
		fmt.Fprintf(a.stdout, "  Description: %s\n", config.Description)
	}

	// Summary
	fmt.Fprintf(a.stdout, "\nConfiguration summary:\n")
	fmt.Fprintf(a.stdout, "  Max steps: %d\n", config.Agent.MaxSteps)
	fmt.Fprintf(a.stdout, "  Initial state: %s\n", config.Agent.InitialState)

	if len(config.Tools.Packs) > 0 {
		fmt.Fprintf(a.stdout, "  Tool packs: %d\n", len(config.Tools.Packs))
		for _, pack := range config.Tools.Packs {
			fmt.Fprintf(a.stdout, "    - %s (v%s)\n", pack.Name, pack.Version)
		}
	}

	if len(config.Tools.Inline) > 0 {
		fmt.Fprintf(a.stdout, "  Inline tools: %d\n", len(config.Tools.Inline))
		for _, tool := range config.Tools.Inline {
			fmt.Fprintf(a.stdout, "    - %s\n", tool.Name)
		}
	}

	if len(config.Tools.Eligibility) > 0 {
		fmt.Fprintf(a.stdout, "  Eligibility rules: %d states\n", len(config.Tools.Eligibility))
	}

	if len(config.Policy.Budgets) > 0 {
		fmt.Fprintf(a.stdout, "  Budgets:\n")
		for name, limit := range config.Policy.Budgets {
			fmt.Fprintf(a.stdout, "    - %s: %d\n", name, limit)
		}
	}

	if config.Policy.RateLimit.Enabled {
		fmt.Fprintf(a.stdout, "  Rate limiting: enabled (rate=%d, burst=%d)\n",
			config.Policy.RateLimit.Rate, config.Policy.RateLimit.Burst)
	}

	if config.Notification.Enabled {
		fmt.Fprintf(a.stdout, "  Notifications: enabled (%d endpoints)\n", len(config.Notification.Endpoints))
	}

	return nil
}

// showConfigSchema displays the JSON schema for configuration.
func (a *App) showConfigSchema() error {
	schemaJSON, err := api.ConfigSchemaJSON()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	fmt.Fprintln(a.stdout, schemaJSON)
	return nil
}
