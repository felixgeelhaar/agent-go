package shell_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/pack/shell"
)

func TestNew(t *testing.T) {
	pack, err := shell.New(
		shell.WithAllowedCommands("echo", "ls"),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}

	tools := pack.Tools
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	// Verify expected tools
	toolNames := make(map[string]bool)
	for _, tl := range tools {
		toolNames[tl.Name()] = true
	}

	expectedTools := []string{"shell_exec", "shell_env"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestShellExec_AllowedCommand(t *testing.T) {
	pack, err := shell.New(
		shell.WithAllowedCommands("echo"),
		shell.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	if execTool == nil {
		t.Fatal("shell_exec tool not found")
	}

	// Execute echo command
	input := json.RawMessage(`{"command": "echo", "args": ["hello", "world"]}`)
	result, err := execTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("shell_exec failed: %v", err)
	}

	// Verify output
	var output struct {
		Stdout   string `json:"stdout"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	expected := "hello world"
	if !strings.Contains(output.Stdout, expected) {
		t.Errorf("expected stdout to contain %q, got %q", expected, output.Stdout)
	}

	if output.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", output.ExitCode)
	}
}

func TestShellExec_BlockedCommand(t *testing.T) {
	pack, err := shell.New(
		shell.WithAllowedCommands("echo"),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	// Try to execute a command not in allowed list
	input := json.RawMessage(`{"command": "cat", "args": ["/etc/passwd"]}`)
	_, err = execTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
}

func TestShellExec_ExplicitlyBlockedCommand(t *testing.T) {
	pack, err := shell.New(
		shell.WithBlockedCommands("rm"),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	// Try to execute blocked command
	input := json.RawMessage(`{"command": "rm", "args": ["-rf", "/"]}`)
	_, err = execTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
}

func TestShellEnv(t *testing.T) {
	pack, err := shell.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_env tool
	var envTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_env" {
			envTool = tl
			break
		}
	}

	if envTool == nil {
		t.Fatal("shell_env tool not found")
	}

	// Execute env command
	input := json.RawMessage(`{}`)
	result, err := envTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("shell_env failed: %v", err)
	}

	// Verify output contains environment variables
	// Note: The output uses "variables" not "environment"
	var output struct {
		Variables map[string]string `json:"variables"`
		Count     int               `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// PATH should be in environment on all platforms
	if _, ok := output.Variables["PATH"]; !ok {
		t.Error("expected PATH in environment variables")
	}

	// Verify count matches
	if output.Count != len(output.Variables) {
		t.Errorf("count mismatch: expected %d, got %d", len(output.Variables), output.Count)
	}
}

func TestShellExec_WithEnvironment(t *testing.T) {
	pack, err := shell.New(
		shell.WithAllowedCommands("env", "sh"),
		shell.WithEnvironment(map[string]string{
			"CUSTOM_VAR": "custom_value",
		}),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	// Skip on Windows - different env handling
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	// Execute command that prints environment
	input := json.RawMessage(`{"command": "sh", "args": ["-c", "echo $CUSTOM_VAR"]}`)
	result, err := execTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("shell_exec failed: %v", err)
	}

	var output struct {
		Stdout string `json:"stdout"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !strings.Contains(output.Stdout, "custom_value") {
		t.Errorf("expected custom_value in output, got %q", output.Stdout)
	}
}

func TestShellExec_Timeout(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	pack, err := shell.New(
		shell.WithAllowedCommands("sleep", "sh"),
		shell.WithTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	// Execute sleep command that should timeout
	// Note: The implementation may not return an error for timeout
	// because killed processes are treated as normal exits.
	// Instead, we verify that the execution completes quickly.
	start := time.Now()
	input := json.RawMessage(`{"command": "sleep", "args": ["10"]}`)
	_, _ = execTool.Execute(context.Background(), input)
	elapsed := time.Since(start)

	// The command should complete within a reasonable time (timeout + buffer)
	// If it ran for the full 10 seconds, something is wrong
	if elapsed > 2*time.Second {
		t.Errorf("command took too long (%v), timeout may not be working", elapsed)
	}
}

func TestDefaultBlockedCommands(t *testing.T) {
	blocked := shell.DefaultBlockedCommands()

	// Verify some dangerous commands are blocked by default
	expectedBlocked := []string{"rm", "mkfs", "dd", "shutdown", "reboot"}
	for _, cmd := range expectedBlocked {
		found := false
		for _, b := range blocked {
			if b == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be in default blocked commands", cmd)
		}
	}
}

func TestDefaultBlockedPatterns(t *testing.T) {
	patterns := shell.DefaultBlockedPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected default blocked patterns, got none")
	}
}

func TestShellExec_WithStdin(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	pack, err := shell.New(
		shell.WithAllowedCommands("cat", "sh"),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_exec tool
	var execTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_exec" {
			execTool = tl
			break
		}
	}

	// Execute cat with stdin
	input := json.RawMessage(`{"command": "cat", "stdin": "hello from stdin"}`)
	result, err := execTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("shell_exec failed: %v", err)
	}

	var output struct {
		Stdout string `json:"stdout"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !strings.Contains(output.Stdout, "hello from stdin") {
		t.Errorf("expected 'hello from stdin' in output, got %q", output.Stdout)
	}
}

func TestShellEnv_WithFilter(t *testing.T) {
	pack, err := shell.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find shell_env tool
	var envTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "shell_env" {
			envTool = tl
			break
		}
	}

	// Execute env command with filter
	input := json.RawMessage(`{"filter": "PATH"}`)
	result, err := envTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("shell_env failed: %v", err)
	}

	var output struct {
		Variables map[string]string `json:"variables"`
		Count     int               `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should have fewer variables with filter
	if output.Count == 0 {
		t.Error("expected at least one variable matching PATH filter")
	}

	// Verify PATH is in the filtered results
	found := false
	for key := range output.Variables {
		if strings.Contains(strings.ToUpper(key), "PATH") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected PATH-related variable in filtered results")
	}
}
