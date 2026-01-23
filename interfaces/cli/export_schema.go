package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// exportSchemaOptions holds options for the export-schema command.
type exportSchemaOptions struct {
	outputPath string
}

// newExportSchemaCmd creates the export-schema command.
func (a *App) newExportSchemaCmd() *cobra.Command {
	opts := &exportSchemaOptions{}

	cmd := &cobra.Command{
		Use:   "export-schema",
		Short: "Export the configuration JSON schema",
		Long: `Export the JSON Schema for agent configuration files.

The exported schema can be used for:
  - IDE validation and autocompletion
  - CI/CD configuration validation
  - Documentation generation

The schema follows JSON Schema draft 2020-12.

Examples:
  # Export schema to stdout
  agent export-schema

  # Export schema to a file
  agent export-schema -o schema.json

  # Use with VS Code
  # Add to .vscode/settings.json:
  # "yaml.schemas": {
  #   "./schema.json": ["agent*.yaml", "config.yaml"]
  # }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exportSchema(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.outputPath, "output", "o", "", "Output file path (default: stdout)")

	return cmd
}

// exportSchema exports the configuration JSON schema.
func (a *App) exportSchema(opts *exportSchemaOptions) error {
	schemaJSON, err := api.ConfigSchemaJSON()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	if opts.outputPath == "" {
		// Write to stdout
		_, _ = fmt.Fprintln(a.stdout, schemaJSON)
		return nil
	}

	// Write to file with restrictive permissions (G306)
	if err := os.WriteFile(opts.outputPath, []byte(schemaJSON), 0600); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	_, _ = fmt.Fprintf(a.stdout, "Schema exported to %s\n", opts.outputPath)
	return nil
}
