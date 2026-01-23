package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApp_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"version"})
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "agent-go version") {
		t.Errorf("version output missing 'agent-go version', got: %s", output)
	}
}

func TestApp_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"--help"})
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "state-driven agent runtime") {
		t.Errorf("help output missing description, got: %s", output)
	}
	if !strings.Contains(output, "run") {
		t.Errorf("help output missing 'run' command, got: %s", output)
	}
	if !strings.Contains(output, "validate") {
		t.Errorf("help output missing 'validate' command, got: %s", output)
	}
}

func TestApp_Validate(t *testing.T) {
	// Create a temporary config file
	content := `
name: test-agent
version: "1.0"
agent:
  max_steps: 50
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"validate", "-c", configPath})
	if err != nil {
		t.Fatalf("validate command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "valid") {
		t.Errorf("validate output missing 'valid', got: %s", output)
	}
}

func TestApp_ValidateInvalid(t *testing.T) {
	// Create an invalid config file
	content := `
name: ""
version: ""
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"validate", "-c", configPath})
	if err == nil {
		t.Fatal("validate command should fail for invalid config")
	}
}

func TestApp_ValidateShowSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"validate", "--schema"})
	if err != nil {
		t.Fatalf("validate --schema failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "$schema") {
		t.Errorf("schema output missing '$schema', got: %s", output)
	}
	if !strings.Contains(output, "Agent Configuration") {
		t.Errorf("schema output missing 'Agent Configuration', got: %s", output)
	}
}

func TestApp_ListPacks(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
tools:
  packs:
    - name: fileops
      version: "1.0"
      enabled:
        - read_file
  inline:
    - name: echo
      description: Echo input
      handler:
        type: exec
        command: echo
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"list-packs", "-c", configPath})
	if err != nil {
		t.Fatalf("list-packs command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "fileops") {
		t.Errorf("list-packs output missing 'fileops', got: %s", output)
	}
	if !strings.Contains(output, "echo") {
		t.Errorf("list-packs output missing 'echo', got: %s", output)
	}
}

func TestApp_Inspect(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
description: A test agent
agent:
  max_steps: 50
policy:
  budgets:
    tool_calls: 100
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"inspect", "-c", configPath})
	if err != nil {
		t.Fatalf("inspect command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "test-agent") {
		t.Errorf("inspect output missing 'test-agent', got: %s", output)
	}
	if !strings.Contains(output, "A test agent") {
		t.Errorf("inspect output missing description, got: %s", output)
	}
}

func TestApp_InspectJSON(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"inspect", "-c", configPath, "--json"})
	if err != nil {
		t.Fatalf("inspect --json failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"name": "test-agent"`) {
		t.Errorf("inspect JSON output missing name, got: %s", output)
	}
}

func TestApp_ExportSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"export-schema"})
	if err != nil {
		t.Fatalf("export-schema failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "$schema") {
		t.Errorf("export-schema output missing '$schema', got: %s", output)
	}
}

func TestApp_ExportSchemaToFile(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"export-schema", "-o", schemaPath})
	if err != nil {
		t.Fatalf("export-schema -o failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema file: %v", err)
	}

	if !strings.Contains(string(data), "$schema") {
		t.Errorf("schema file missing '$schema'")
	}
}

func TestApp_RunDryRun(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
agent:
  max_steps: 50
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"run", "-c", configPath, "--dry-run", "Test goal"})
	if err != nil {
		t.Fatalf("run --dry-run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "validated successfully") {
		t.Errorf("run --dry-run output missing 'validated successfully', got: %s", output)
	}
}

func TestApp_RunNoGoal(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"run", "-c", configPath})
	if err == nil {
		t.Fatal("run without goal should fail")
	}
	if !strings.Contains(err.Error(), "no goal specified") {
		t.Errorf("error should mention 'no goal specified', got: %v", err)
	}
}

func TestApp_Run(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
agent:
  max_steps: 50
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"run", "-c", configPath, "Test goal"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Run completed") {
		t.Errorf("run output missing 'Run completed', got: %s", output)
	}
}

func TestApp_RunWithJSON(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := New().WithOutput(&stdout, &stderr)

	err := app.ExecuteWithArgs(context.Background(), []string{"run", "-c", configPath, "--json", "Test goal"})
	if err != nil {
		t.Fatalf("run --json failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"run_id"`) {
		t.Errorf("run JSON output missing 'run_id', got: %s", output)
	}
	if !strings.Contains(output, `"state"`) {
		t.Errorf("run JSON output missing 'state', got: %s", output)
	}
}
