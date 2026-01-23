package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoader_LoadFile_YAML(t *testing.T) {
	content := `
name: test-agent
version: "1.0"
description: Test agent
agent:
  max_steps: 50
  initial_state: intake
tools:
  eligibility:
    explore:
      - read_file
      - list_dir
    act:
      - write_file
policy:
  budgets:
    tool_calls: 100
`
	// Write to temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if cfg.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", cfg.Name)
	}
	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}
	if cfg.Agent.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want 50", cfg.Agent.MaxSteps)
	}
	if len(cfg.Tools.Eligibility["explore"]) != 2 {
		t.Errorf("Eligibility[explore] has %d tools, want 2", len(cfg.Tools.Eligibility["explore"]))
	}
	if cfg.Policy.Budgets["tool_calls"] != 100 {
		t.Errorf("Budgets[tool_calls] = %d, want 100", cfg.Policy.Budgets["tool_calls"])
	}
}

func TestLoader_LoadFile_JSON(t *testing.T) {
	content := `{
  "name": "test-agent",
  "version": "1.0",
  "agent": {
    "max_steps": 50
  }
}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if cfg.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", cfg.Name)
	}
	if cfg.Agent.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want 50", cfg.Agent.MaxSteps)
	}
}

func TestLoader_LoadFile_NotFound(t *testing.T) {
	loader := NewLoader()
	_, err := loader.LoadFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("LoadFile() should return error for nonexistent file")
	}
}

func TestLoader_LoadFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.txt")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loader := NewLoader()
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Error("LoadFile() should return error for unsupported format")
	}
}

func TestLoader_LoadString(t *testing.T) {
	content := `name: test-agent
version: "1.0"
`
	loader := NewLoader()
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	if cfg.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", cfg.Name)
	}
}

func TestLoader_EnvExpansion(t *testing.T) {
	os.Setenv("TEST_AGENT_NAME", "env-agent")
	defer os.Unsetenv("TEST_AGENT_NAME")

	content := `
name: ${TEST_AGENT_NAME}
version: "1.0"
`
	loader := NewLoader()
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	if cfg.Name != "env-agent" {
		t.Errorf("Name = %s, want env-agent", cfg.Name)
	}
}

func TestLoader_EnvExpansionWithDefault(t *testing.T) {
	os.Unsetenv("UNSET_VAR")

	content := `
name: ${UNSET_VAR:-default-agent}
version: "1.0"
`
	loader := NewLoader()
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	if cfg.Name != "default-agent" {
		t.Errorf("Name = %s, want default-agent", cfg.Name)
	}
}

func TestLoader_EnvExpansionStrict(t *testing.T) {
	os.Unsetenv("MISSING_VAR")

	content := `
name: ${MISSING_VAR}
version: "1.0"
`
	loader := NewLoaderWithOptions(WithStrictEnv(true))
	_, err := loader.LoadString(content, FormatYAML)
	if err == nil {
		t.Error("LoadString() should return error for missing env var in strict mode")
	}
}

func TestLoader_EnvExpansionDisabled(t *testing.T) {
	os.Setenv("TEST_VAR", "expanded")
	defer os.Unsetenv("TEST_VAR")

	content := `
name: ${TEST_VAR}
version: "1.0"
`
	loader := NewLoaderWithOptions(WithEnvExpansion(false), WithValidation(false))
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	// Should NOT expand
	if cfg.Name != "${TEST_VAR}" {
		t.Errorf("Name = %s, want ${TEST_VAR} (unexpanded)", cfg.Name)
	}
}

func TestLoader_ValidationFailed(t *testing.T) {
	content := `
name: ""
version: ""
`
	loader := NewLoader()
	_, err := loader.LoadString(content, FormatYAML)
	if err == nil {
		t.Error("LoadString() should return error for invalid config")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("error should mention validation, got: %v", err)
	}
}

func TestLoader_ValidationDisabled(t *testing.T) {
	content := `
name: ""
version: ""
`
	loader := NewLoaderWithOptions(WithValidation(false))
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v (validation should be disabled)", err)
	}

	if cfg.Name != "" {
		t.Errorf("Name = %s, want empty", cfg.Name)
	}
}

func TestLoader_InvalidYAML(t *testing.T) {
	content := `
name: test
  invalid: yaml indentation
`
	loader := NewLoaderWithOptions(WithValidation(false))
	_, err := loader.LoadString(content, FormatYAML)
	if err == nil {
		t.Error("LoadString() should return error for invalid YAML")
	}
}

func TestLoader_InvalidJSON(t *testing.T) {
	content := `{"name": invalid json}`
	loader := NewLoaderWithOptions(WithValidation(false))
	_, err := loader.LoadString(content, FormatJSON)
	if err == nil {
		t.Error("LoadString() should return error for invalid JSON")
	}
}

func TestLoader_ComplexConfig(t *testing.T) {
	content := `
name: complex-agent
version: "1.0"
description: A complex test agent
agent:
  max_steps: 100
  initial_state: intake
tools:
  packs:
    - name: fileops
      version: "1.0"
      config:
        root_dir: /tmp
      enabled:
        - read_file
        - write_file
  inline:
    - name: echo
      description: Echo input
      annotations:
        read_only: true
        risk_level: none
      handler:
        type: exec
        command: echo
        args:
          - "hello"
  eligibility:
    explore:
      - read_file
    act:
      - write_file
      - echo
policy:
  budgets:
    tool_calls: 100
    tokens: 10000
  approval:
    mode: auto
    require_for_destructive: true
  rate_limit:
    enabled: true
    rate: 10
    burst: 20
resilience:
  timeout: 30s
  retry:
    enabled: true
    max_attempts: 3
    initial_delay: 1s
    multiplier: 2.0
  circuit_breaker:
    enabled: true
    threshold: 5
    timeout: 30s
  bulkhead:
    enabled: true
    max_concurrent: 10
notification:
  enabled: true
  endpoints:
    - name: slack
      url: https://hooks.slack.com/test
      enabled: true
      secret: webhook-secret
  batching:
    enabled: true
    max_size: 100
    max_wait: 5s
variables:
  env: test
  debug: true
`
	loader := NewLoader()
	cfg, err := loader.LoadString(content, FormatYAML)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	// Verify various fields
	if cfg.Name != "complex-agent" {
		t.Errorf("Name = %s, want complex-agent", cfg.Name)
	}
	if len(cfg.Tools.Packs) != 1 {
		t.Errorf("Tools.Packs has %d packs, want 1", len(cfg.Tools.Packs))
	}
	if cfg.Tools.Packs[0].Name != "fileops" {
		t.Errorf("Pack name = %s, want fileops", cfg.Tools.Packs[0].Name)
	}
	if len(cfg.Tools.Inline) != 1 {
		t.Errorf("Tools.Inline has %d tools, want 1", len(cfg.Tools.Inline))
	}
	if cfg.Policy.RateLimit.Rate != 10 {
		t.Errorf("RateLimit.Rate = %d, want 10", cfg.Policy.RateLimit.Rate)
	}
	if cfg.Resilience.Timeout.Duration().Seconds() != 30 {
		t.Errorf("Timeout = %v, want 30s", cfg.Resilience.Timeout)
	}
	if len(cfg.Notification.Endpoints) != 1 {
		t.Errorf("Notification.Endpoints has %d endpoints, want 1", len(cfg.Notification.Endpoints))
	}
	if cfg.Variables["env"] != "test" {
		t.Errorf("Variables[env] = %v, want test", cfg.Variables["env"])
	}
}
