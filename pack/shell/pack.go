// Package shell provides shell command execution tools with security controls.
package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Config configures the shell pack.
type Config struct {
	// AllowedCommands is a list of commands that are allowed to run.
	// If empty, all commands are allowed (use with caution).
	AllowedCommands []string

	// BlockedCommands is a list of commands that are explicitly blocked.
	// Takes precedence over AllowedCommands.
	BlockedCommands []string

	// Timeout for command execution.
	Timeout time.Duration

	// MaxOutputSize limits the size of command output (bytes).
	MaxOutputSize int64

	// WorkingDir is the default working directory for commands.
	WorkingDir string

	// Environment is additional environment variables.
	Environment map[string]string

	// Shell is the shell to use (default: /bin/sh).
	Shell string

	// AllowScripts enables the shell_script tool.
	AllowScripts bool

	// BlockedPatterns are regex patterns that block command execution.
	BlockedPatterns []string

	// compiledPatterns are pre-compiled regex patterns.
	compiledPatterns []*regexp.Regexp
}

// Option configures the shell pack.
type Option func(*Config)

// WithAllowedCommands sets the list of allowed commands.
func WithAllowedCommands(commands ...string) Option {
	return func(c *Config) {
		c.AllowedCommands = commands
	}
}

// WithBlockedCommands sets the list of blocked commands.
func WithBlockedCommands(commands ...string) Option {
	return func(c *Config) {
		c.BlockedCommands = commands
	}
}

// WithTimeout sets the command timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithMaxOutputSize sets the maximum output size.
func WithMaxOutputSize(size int64) Option {
	return func(c *Config) {
		c.MaxOutputSize = size
	}
}

// WithWorkingDir sets the default working directory.
func WithWorkingDir(dir string) Option {
	return func(c *Config) {
		c.WorkingDir = dir
	}
}

// WithEnvironment sets additional environment variables.
func WithEnvironment(env map[string]string) Option {
	return func(c *Config) {
		c.Environment = env
	}
}

// WithShell sets the shell to use.
func WithShell(shell string) Option {
	return func(c *Config) {
		c.Shell = shell
	}
}

// WithScripts enables the shell_script tool.
func WithScripts() Option {
	return func(c *Config) {
		c.AllowScripts = true
	}
}

// WithBlockedPatterns sets regex patterns that block command execution.
func WithBlockedPatterns(patterns ...string) Option {
	return func(c *Config) {
		c.BlockedPatterns = patterns
	}
}

// DefaultBlockedCommands returns a sensible list of blocked commands.
func DefaultBlockedCommands() []string {
	return []string{
		"rm", "rmdir", "dd", "mkfs", "fdisk", "parted",
		"shutdown", "reboot", "halt", "poweroff", "init",
		"kill", "killall", "pkill",
		"chmod", "chown", "chgrp",
		"su", "sudo", "doas",
		"passwd", "useradd", "userdel", "groupadd", "groupdel",
		"mount", "umount",
		"iptables", "ip6tables", "nft", "ufw",
		"systemctl", "service", "rc-service",
	}
}

// DefaultBlockedPatterns returns sensible blocked patterns.
func DefaultBlockedPatterns() []string {
	return []string{
		`>\s*/dev/`,           // Writing to devices
		`\|\s*sh\b`,           // Piping to shell
		`\|\s*bash\b`,         // Piping to bash
		`;\s*rm\s`,            // Chained rm
		`&&\s*rm\s`,           // Chained rm
		`\$\(.*\)`,            // Command substitution (can be enabled if needed)
		"`.*`",                // Backtick command substitution
		`>\s*/etc/`,           // Writing to /etc
		`>\s*/usr/`,           // Writing to /usr
		`>\s*/var/`,           // Writing to /var
		`curl.*\|\s*(sh|bash)`, // Piped curl execution
		`wget.*\|\s*(sh|bash)`, // Piped wget execution
	}
}

// New creates the shell pack.
func New(opts ...Option) (*pack.Pack, error) {
	cfg := Config{
		BlockedCommands: DefaultBlockedCommands(),
		BlockedPatterns: DefaultBlockedPatterns(),
		Timeout:         30 * time.Second,
		MaxOutputSize:   1024 * 1024, // 1MB default
		Shell:           "/bin/sh",
		AllowScripts:    false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Compile blocked patterns
	for _, pattern := range cfg.BlockedPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid blocked pattern %q: %w", pattern, err)
		}
		cfg.compiledPatterns = append(cfg.compiledPatterns, re)
	}

	// Validate working directory
	if cfg.WorkingDir != "" {
		info, err := os.Stat(cfg.WorkingDir)
		if err != nil {
			return nil, fmt.Errorf("invalid working directory: %w", err)
		}
		if !info.IsDir() {
			return nil, errors.New("working directory is not a directory")
		}
	}

	builder := pack.NewBuilder("shell").
		WithDescription("Shell command execution with security controls").
		WithVersion("1.0.0").
		AddTools(
			execTool(&cfg),
			envTool(&cfg),
		).
		AllowInState(agent.StateExplore, "shell_env").
		AllowInState(agent.StateValidate, "shell_env")

	if cfg.AllowScripts {
		builder = builder.AddTools(scriptTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "shell_exec", "shell_script", "shell_env")
	} else {
		builder = builder.AllowInState(agent.StateAct, "shell_exec", "shell_env")
	}

	return builder.Build(), nil
}

// isCommandAllowed checks if a command is allowed to run.
func isCommandAllowed(cfg *Config, command string) error {
	// Parse command to get the base command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return errors.New("empty command")
	}

	baseCmd := parts[0]

	// Check blocked commands
	for _, blocked := range cfg.BlockedCommands {
		if baseCmd == blocked {
			return fmt.Errorf("command %q is blocked", baseCmd)
		}
	}

	// Check blocked patterns
	for _, pattern := range cfg.compiledPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command matches blocked pattern")
		}
	}

	// Check allowed commands (if specified)
	if len(cfg.AllowedCommands) > 0 {
		allowed := false
		for _, cmd := range cfg.AllowedCommands {
			if baseCmd == cmd {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %q is not in allowed list", baseCmd)
		}
	}

	return nil
}

// --- shell_exec tool ---

type execInput struct {
	Command    string            `json:"command"`
	Args       []string          `json:"args,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Stdin      string            `json:"stdin,omitempty"`
}

type execOutput struct {
	Command   string `json:"command"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Duration  string `json:"duration"`
	Truncated bool   `json:"truncated,omitempty"`
}

func execTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("shell_exec").
		WithDescription("Execute a shell command").
		WithRiskLevel(tool.RiskHigh).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in execInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate command
			fullCommand := in.Command
			if len(in.Args) > 0 {
				fullCommand = in.Command + " " + strings.Join(in.Args, " ")
			}

			if err := isCommandAllowed(cfg, fullCommand); err != nil {
				return tool.Result{}, err
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Build command
			var cmd *exec.Cmd
			if len(in.Args) > 0 {
				cmd = exec.CommandContext(ctx, in.Command, in.Args...) // #nosec G204 -- command validated above
			} else {
				cmd = exec.CommandContext(ctx, cfg.Shell, "-c", in.Command) // #nosec G204 -- command validated above
			}

			// Set working directory
			if in.WorkingDir != "" {
				cmd.Dir = in.WorkingDir
			} else if cfg.WorkingDir != "" {
				cmd.Dir = cfg.WorkingDir
			}

			// Set environment
			cmd.Env = os.Environ()
			for k, v := range cfg.Environment {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			for k, v := range in.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			// Set stdin if provided
			if in.Stdin != "" {
				cmd.Stdin = strings.NewReader(in.Stdin)
			}

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Execute
			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			// Get exit code
			exitCode := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
				err = nil // Exit code != 0 is not an error
			}

			// Truncate output if needed
			stdoutStr := stdout.String()
			stderrStr := stderr.String()
			truncated := false

			if int64(len(stdoutStr)) > cfg.MaxOutputSize {
				stdoutStr = stdoutStr[:cfg.MaxOutputSize]
				truncated = true
			}
			if int64(len(stderrStr)) > cfg.MaxOutputSize {
				stderrStr = stderrStr[:cfg.MaxOutputSize]
				truncated = true
			}

			if err != nil {
				return tool.Result{}, err
			}

			out := execOutput{
				Command:   fullCommand,
				Stdout:    stdoutStr,
				Stderr:    stderrStr,
				ExitCode:  exitCode,
				Duration:  duration.String(),
				Truncated: truncated,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- shell_script tool ---

type scriptInput struct {
	Script     string            `json:"script"`
	Shell      string            `json:"shell,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

type scriptOutput struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Duration  string `json:"duration"`
	Truncated bool   `json:"truncated,omitempty"`
}

func scriptTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("shell_script").
		WithDescription("Execute a multi-line shell script").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in scriptInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate each line of the script
			lines := strings.Split(in.Script, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if err := isCommandAllowed(cfg, line); err != nil {
					return tool.Result{}, fmt.Errorf("script contains blocked command: %w", err)
				}
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Determine shell
			shell := cfg.Shell
			if in.Shell != "" {
				shell = in.Shell
			}

			// Build command
			cmd := exec.CommandContext(ctx, shell) // #nosec G204 -- shell is configurable
			cmd.Stdin = strings.NewReader(in.Script)

			// Set working directory
			if in.WorkingDir != "" {
				cmd.Dir = in.WorkingDir
			} else if cfg.WorkingDir != "" {
				cmd.Dir = cfg.WorkingDir
			}

			// Set environment
			cmd.Env = os.Environ()
			for k, v := range cfg.Environment {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			for k, v := range in.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Execute
			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			// Get exit code
			exitCode := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
				err = nil
			}

			// Truncate output if needed
			stdoutStr := stdout.String()
			stderrStr := stderr.String()
			truncated := false

			if int64(len(stdoutStr)) > cfg.MaxOutputSize {
				stdoutStr = stdoutStr[:cfg.MaxOutputSize]
				truncated = true
			}
			if int64(len(stderrStr)) > cfg.MaxOutputSize {
				stderrStr = stderrStr[:cfg.MaxOutputSize]
				truncated = true
			}

			if err != nil {
				return tool.Result{}, err
			}

			out := scriptOutput{
				Stdout:    stdoutStr,
				Stderr:    stderrStr,
				ExitCode:  exitCode,
				Duration:  duration.String(),
				Truncated: truncated,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- shell_env tool ---

type envInput struct {
	Filter string `json:"filter,omitempty"`
}

type envOutput struct {
	Variables map[string]string `json:"variables"`
	Count     int               `json:"count"`
}

func envTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("shell_env").
		WithDescription("Get environment variables").
		ReadOnly().
		Cacheable().
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in envInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Collect environment variables
			vars := make(map[string]string)
			for _, env := range os.Environ() {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) != 2 {
					continue
				}

				key := parts[0]
				value := parts[1]

				// Apply filter
				if in.Filter != "" {
					if !strings.Contains(strings.ToLower(key), strings.ToLower(in.Filter)) &&
						!strings.Contains(strings.ToLower(value), strings.ToLower(in.Filter)) {
						continue
					}
				}

				// Redact sensitive values
				lowerKey := strings.ToLower(key)
				if strings.Contains(lowerKey, "password") ||
					strings.Contains(lowerKey, "secret") ||
					strings.Contains(lowerKey, "token") ||
					strings.Contains(lowerKey, "key") ||
					strings.Contains(lowerKey, "credential") {
					value = "[REDACTED]"
				}

				vars[key] = value
			}

			// Add configured environment
			for k, v := range cfg.Environment {
				if in.Filter != "" {
					if !strings.Contains(strings.ToLower(k), strings.ToLower(in.Filter)) &&
						!strings.Contains(strings.ToLower(v), strings.ToLower(in.Filter)) {
						continue
					}
				}
				vars[k] = v
			}

			out := envOutput{
				Variables: vars,
				Count:     len(vars),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
