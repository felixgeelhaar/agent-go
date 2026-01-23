package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// listPacksOptions holds options for the list-packs command.
type listPacksOptions struct {
	configPath string
	verbose    bool
}

// newListPacksCmd creates the list-packs command.
func (a *App) newListPacksCmd() *cobra.Command {
	opts := &listPacksOptions{}

	cmd := &cobra.Command{
		Use:   "list-packs",
		Short: "List configured tool packs",
		Long: `List all tool packs configured in the agent configuration file.

Tool packs are reusable collections of tools that can be enabled or disabled
as a unit. This command shows which packs are configured, their versions,
and which tools are enabled or disabled.

Examples:
  # List tool packs from a configuration
  agent list-packs -c config.yaml

  # Verbose output with tool details
  agent list-packs -c config.yaml -v`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.listPacks(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configPath, "config", "c", "", "Path to configuration file (required)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Show detailed information")

	_ = cmd.MarkFlagRequired("config")

	return cmd
}

// listPacks lists the tool packs from configuration.
func (a *App) listPacks(opts *listPacksOptions) error {
	loader := api.NewConfigLoader()
	config, err := loader.LoadFile(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if len(config.Tools.Packs) == 0 && len(config.Tools.Inline) == 0 {
		_, _ = fmt.Fprintf(a.stdout, "No tool packs or inline tools configured.\n")
		return nil
	}

	// Tool packs
	if len(config.Tools.Packs) > 0 {
		_, _ = fmt.Fprintf(a.stdout, "Tool Packs (%d):\n", len(config.Tools.Packs))
		for _, pack := range config.Tools.Packs {
			_, _ = fmt.Fprintf(a.stdout, "\n  %s", pack.Name)
			if pack.Version != "" {
				_, _ = fmt.Fprintf(a.stdout, " (v%s)", pack.Version)
			}
			_, _ = fmt.Fprintf(a.stdout, "\n")

			if opts.verbose {
				if len(pack.Config) > 0 {
					_, _ = fmt.Fprintf(a.stdout, "    Configuration:\n")
					for k, v := range pack.Config {
						_, _ = fmt.Fprintf(a.stdout, "      %s: %v\n", k, v)
					}
				}
			}

			if len(pack.Enabled) > 0 {
				_, _ = fmt.Fprintf(a.stdout, "    Enabled tools: %v\n", pack.Enabled)
			}
			if len(pack.Disabled) > 0 {
				_, _ = fmt.Fprintf(a.stdout, "    Disabled tools: %v\n", pack.Disabled)
			}
		}
	}

	// Inline tools
	if len(config.Tools.Inline) > 0 {
		_, _ = fmt.Fprintf(a.stdout, "\nInline Tools (%d):\n", len(config.Tools.Inline))
		for _, tool := range config.Tools.Inline {
			_, _ = fmt.Fprintf(a.stdout, "\n  %s\n", tool.Name)
			if tool.Description != "" {
				_, _ = fmt.Fprintf(a.stdout, "    Description: %s\n", tool.Description)
			}

			if opts.verbose {
				_, _ = fmt.Fprintf(a.stdout, "    Handler: %s\n", tool.Handler.Type)
				if tool.Handler.Command != "" {
					_, _ = fmt.Fprintf(a.stdout, "    Command: %s\n", tool.Handler.Command)
				}
				if len(tool.Handler.Args) > 0 {
					_, _ = fmt.Fprintf(a.stdout, "    Args: %v\n", tool.Handler.Args)
				}

				// Annotations
				if tool.Annotations.ReadOnly {
					_, _ = fmt.Fprintf(a.stdout, "    ReadOnly: true\n")
				}
				if tool.Annotations.Destructive {
					_, _ = fmt.Fprintf(a.stdout, "    Destructive: true\n")
				}
				if tool.Annotations.Idempotent {
					_, _ = fmt.Fprintf(a.stdout, "    Idempotent: true\n")
				}
				if tool.Annotations.RiskLevel != "" {
					_, _ = fmt.Fprintf(a.stdout, "    Risk Level: %s\n", tool.Annotations.RiskLevel)
				}
			}
		}
	}

	// Eligibility summary
	if len(config.Tools.Eligibility) > 0 {
		_, _ = fmt.Fprintf(a.stdout, "\nTool Eligibility by State:\n")
		for state, tools := range config.Tools.Eligibility {
			_, _ = fmt.Fprintf(a.stdout, "  %s: %v\n", state, tools)
		}
	}

	return nil
}
