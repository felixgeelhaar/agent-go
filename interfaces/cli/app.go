// Package cli provides a command-line interface for the agent-go runtime.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// Version information set at build time.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// App represents the CLI application.
type App struct {
	root   *cobra.Command
	stdout io.Writer
	stderr io.Writer
}

// New creates a new CLI application.
func New() *App {
	app := &App{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	app.root = &cobra.Command{
		Use:   "agent",
		Short: "State-driven agent runtime for Go",
		Long: `agent-go is a state-driven agent runtime that enables developers to build
trustworthy, adaptable AI-powered systems by designing the structure and
constraints of agent behavior rather than scripting intelligence with prompts.

Key principle: Trust is the product. Intelligence is constrained by design, not hope.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add subcommands
	app.root.AddCommand(
		app.newVersionCmd(),
		app.newValidateCmd(),
		app.newRunCmd(),
		app.newListPacksCmd(),
		app.newInspectCmd(),
		app.newExportSchemaCmd(),
	)

	return app
}

// WithOutput sets custom output writers.
func (a *App) WithOutput(stdout, stderr io.Writer) *App {
	a.stdout = stdout
	a.stderr = stderr
	a.root.SetOut(stdout)
	a.root.SetErr(stderr)
	return a
}

// Execute runs the CLI application.
func (a *App) Execute(ctx context.Context) error {
	// Set up signal handling
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return a.root.ExecuteContext(ctx)
}

// ExecuteWithArgs runs the CLI with specific arguments (useful for testing).
func (a *App) ExecuteWithArgs(ctx context.Context, args []string) error {
	a.root.SetArgs(args)
	return a.Execute(ctx)
}

// newVersionCmd creates the version command.
func (a *App) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(a.stdout, "agent-go version %s\n", Version)
			fmt.Fprintf(a.stdout, "  Git commit: %s\n", GitCommit)
			fmt.Fprintf(a.stdout, "  Build date: %s\n", BuildDate)
		},
	}
}
