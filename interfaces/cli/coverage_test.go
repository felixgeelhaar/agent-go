package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to create a temp config file and return its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return p
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := New().WithOutput(&out, &errOut)
	err = app.ExecuteWithArgs(context.Background(), args)
	return out.String(), errOut.String(), err
}

// ─── inspect.go tests ───

func TestInspect_JSONSections(t *testing.T) {
	cfg := writeConfig(t, `
name: test-agent
version: "1.0"
description: test desc
agent:
  max_steps: 10
  initial_state: intake
  default_goal: do stuff
tools:
  packs:
    - name: mypack
      version: "2.0"
  inline:
    - name: mytool
      description: a tool
      handler:
        type: exec
        command: echo
  eligibility:
    explore:
      - mytool
policy:
  budgets:
    tool_calls: 50
  approval:
    mode: auto
    require_for_destructive: true
  rate_limit:
    enabled: true
    rate: 10
    burst: 20
    per_tool: true
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
    timeout: 60s
  bulkhead:
    enabled: true
    max_concurrent: 10
notification:
  enabled: true
  endpoints:
    - name: webhook1
      url: http://example.com
      enabled: true
    - name: webhook2
      url: http://example.com
      enabled: false
  batching:
    enabled: true
    max_size: 100
    max_wait: 5s
variables:
  env: production
  debug: true
`)

	sections := []string{"all", "agent", "tools", "policy", "resilience", "notification"}
	for _, section := range sections {
		t.Run("json_"+section, func(t *testing.T) {
			out, _, err := runCLI(t, "inspect", "-c", cfg, "--json", "--section", section)
			if err != nil {
				t.Fatalf("inspect --json --section %s failed: %v", section, err)
			}
			if out == "" {
				t.Errorf("expected JSON output for section %s", section)
			}
		})
	}

	// Unknown section
	t.Run("json_unknown", func(t *testing.T) {
		_, _, err := runCLI(t, "inspect", "-c", cfg, "--json", "--section", "bogus")
		if err == nil {
			t.Fatal("expected error for unknown section")
		}
	})
}

func TestInspect_TextSections(t *testing.T) {
	cfg := writeConfig(t, `
name: full-agent
version: "2.0"
description: full test
agent:
  max_steps: 25
  initial_state: intake
  default_goal: my goal
tools:
  packs:
    - name: pack1
      version: "1.0"
  inline:
    - name: tool1
      description: inline tool
      handler:
        type: exec
        command: echo
  eligibility:
    explore:
      - tool1
policy:
  budgets:
    tool_calls: 100
  approval:
    mode: manual
    require_for_destructive: true
  rate_limit:
    enabled: true
    rate: 5
    burst: 10
    per_tool: true
resilience:
  timeout: 10s
  retry:
    enabled: true
    max_attempts: 3
    initial_delay: 500ms
    multiplier: 1.5
  circuit_breaker:
    enabled: true
    threshold: 3
    timeout: 30s
  bulkhead:
    enabled: true
    max_concurrent: 5
notification:
  enabled: true
  endpoints:
    - name: ep1
      url: http://example.com
      enabled: true
    - name: ep2
      url: http://example.com
      enabled: false
  batching:
    enabled: true
    max_size: 50
    max_wait: 2s
variables:
  key1: val1
  key2: val2
`)

	sections := []string{"all", "agent", "tools", "policy", "resilience", "notification"}
	for _, section := range sections {
		t.Run("text_"+section, func(t *testing.T) {
			out, _, err := runCLI(t, "inspect", "-c", cfg, "--section", section)
			if err != nil {
				t.Fatalf("inspect --section %s failed: %v", section, err)
			}
			if out == "" {
				t.Errorf("expected text output for section %s", section)
			}
		})
	}

	// Unknown section in text mode
	t.Run("text_unknown", func(t *testing.T) {
		_, _, err := runCLI(t, "inspect", "-c", cfg, "--section", "bogus")
		if err == nil {
			t.Fatal("expected error for unknown section")
		}
	})
}

func TestInspect_TextToolsSectionNoTools(t *testing.T) {
	cfg := writeConfig(t, `
name: empty-tools
version: "1.0"
`)
	out, _, err := runCLI(t, "inspect", "-c", cfg, "--section", "tools")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No tools configured") {
		t.Errorf("expected 'No tools configured', got: %s", out)
	}
}

func TestInspect_TextNotificationDisabled(t *testing.T) {
	cfg := writeConfig(t, `
name: no-notify
version: "1.0"
notification:
  enabled: false
`)
	out, _, err := runCLI(t, "inspect", "-c", cfg, "--section", "notification")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Enabled: false") {
		t.Errorf("expected 'Enabled: false', got: %s", out)
	}
}

func TestInspect_TextVariablesEmpty(t *testing.T) {
	cfg := writeConfig(t, `
name: no-vars
version: "1.0"
`)
	out, _, err := runCLI(t, "inspect", "-c", cfg, "--section", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Variables section should not appear
	if strings.Contains(out, "Variables") {
		t.Errorf("did not expect Variables section, got: %s", out)
	}
}

func TestInspect_InvalidConfig(t *testing.T) {
	cfg := writeConfig(t, `invalid yaml: [`)
	_, _, err := runCLI(t, "inspect", "-c", cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestInspect_TextResilienceNoTimeout(t *testing.T) {
	cfg := writeConfig(t, `
name: no-timeout
version: "1.0"
resilience:
  retry:
    enabled: false
  circuit_breaker:
    enabled: false
  bulkhead:
    enabled: false
`)
	out, _, err := runCLI(t, "inspect", "-c", cfg, "--section", "resilience")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Timeout:") {
		t.Errorf("did not expect Timeout line, got: %s", out)
	}
}

// ─── list_packs.go tests ───

func TestListPacks_NoPacks(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
`)
	out, _, err := runCLI(t, "list-packs", "-c", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No tool packs") {
		t.Errorf("expected 'No tool packs', got: %s", out)
	}
}

func TestListPacks_Verbose(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
tools:
  packs:
    - name: mypack
      version: "1.0"
      config:
        key1: val1
      enabled:
        - tool_a
      disabled:
        - tool_b
  inline:
    - name: inline_tool
      description: an inline tool
      handler:
        type: exec
        command: ls
        args:
          - -la
      annotations:
        read_only: true
        destructive: true
        idempotent: true
        risk_level: high
  eligibility:
    explore:
      - inline_tool
`)
	out, _, err := runCLI(t, "list-packs", "-c", cfg, "-v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mypack") {
		t.Errorf("expected 'mypack', got: %s", out)
	}
	if !strings.Contains(out, "key1") {
		t.Errorf("expected config key 'key1', got: %s", out)
	}
	if !strings.Contains(out, "inline_tool") {
		t.Errorf("expected 'inline_tool', got: %s", out)
	}
	if !strings.Contains(out, "ReadOnly") {
		t.Errorf("expected 'ReadOnly', got: %s", out)
	}
	if !strings.Contains(out, "Destructive") {
		t.Errorf("expected 'Destructive', got: %s", out)
	}
	if !strings.Contains(out, "Idempotent") {
		t.Errorf("expected 'Idempotent', got: %s", out)
	}
	if !strings.Contains(out, "Risk Level") {
		t.Errorf("expected 'Risk Level', got: %s", out)
	}
	if !strings.Contains(out, "Args") {
		t.Errorf("expected 'Args', got: %s", out)
	}
	if !strings.Contains(out, "Eligibility") {
		t.Errorf("expected 'Eligibility', got: %s", out)
	}
}

func TestListPacks_InvalidConfig(t *testing.T) {
	cfg := writeConfig(t, `not valid yaml: [`)
	_, _, err := runCLI(t, "list-packs", "-c", cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestListPacks_PackWithoutVersion(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
tools:
  packs:
    - name: noversion
`)
	out, _, err := runCLI(t, "list-packs", "-c", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "noversion") {
		t.Errorf("expected 'noversion', got: %s", out)
	}
}

// ─── run.go tests ───

func TestRun_VerboseDryRun(t *testing.T) {
	cfg := writeConfig(t, `
name: verbose-agent
version: "3.0"
description: verbose test
agent:
  max_steps: 20
  initial_state: intake
tools:
  packs:
    - name: pack1
  inline:
    - name: tool1
      description: t
      handler:
        type: exec
        command: echo
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "--dry-run", "-v", "My goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "verbose-agent") {
		t.Errorf("expected agent name in verbose output, got: %s", out)
	}
	if !strings.Contains(out, "validated successfully") {
		t.Errorf("expected 'validated successfully', got: %s", out)
	}
	if !strings.Contains(out, "My goal") {
		t.Errorf("expected goal in output, got: %s", out)
	}
}

func TestRun_DryRunNoGoal(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 10
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "validated successfully") {
		t.Errorf("expected 'validated successfully', got: %s", out)
	}
}

func TestRun_WithDefaultGoal(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
  default_goal: default goal here
`)
	out, _, err := runCLI(t, "run", "-c", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

func TestRun_WithMaxStepsOverride(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 10
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "--max-steps", "100", "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

func TestRun_WithVars(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "--var", "env=prod", "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

func TestRun_VerboseWithGoal(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "-v", "my goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Starting agent run") {
		t.Errorf("expected 'Starting agent run', got: %s", out)
	}
}

func TestRun_WithTimeout(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "--timeout", "30s", "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

func TestRun_WithRateLimit(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
policy:
  rate_limit:
    enabled: true
    rate: 10
    burst: 20
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	cfg := writeConfig(t, `broken: [`)
	_, _, err := runCLI(t, "run", "-c", cfg, "goal")
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestRun_WithEligibility(t *testing.T) {
	cfg := writeConfig(t, `
name: test
version: "1.0"
agent:
  max_steps: 50
tools:
  eligibility:
    explore:
      - some_tool
`)
	out, _, err := runCLI(t, "run", "-c", cfg, "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Run completed") {
		t.Errorf("expected 'Run completed', got: %s", out)
	}
}

// ─── validate.go tests ───

func TestValidate_NoConfigPath(t *testing.T) {
	_, _, err := runCLI(t, "validate")
	if err == nil {
		t.Fatal("expected error when no config path given")
	}
}

func TestValidate_Strict(t *testing.T) {
	cfg := writeConfig(t, `
name: test-agent
version: "1.0"
agent:
  max_steps: 50
`)
	out, _, err := runCLI(t, "validate", "-c", cfg, "--strict")
	if err != nil {
		t.Fatalf("strict validation failed: %v", err)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got: %s", out)
	}
}

func TestValidate_FullOutput(t *testing.T) {
	cfg := writeConfig(t, `
name: full-agent
version: "2.0"
description: full validation test
agent:
  max_steps: 100
  initial_state: intake
tools:
  packs:
    - name: mypack
      version: "1.0"
  inline:
    - name: mytool
      description: a tool
      handler:
        type: exec
        command: echo
  eligibility:
    explore:
      - mytool
policy:
  budgets:
    tool_calls: 200
  rate_limit:
    enabled: true
    rate: 5
    burst: 10
notification:
  enabled: true
  endpoints:
    - name: hook1
      url: http://example.com
      enabled: true
`)
	out, _, err := runCLI(t, "validate", "-c", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "full-agent") {
		t.Errorf("expected agent name, got: %s", out)
	}
	if !strings.Contains(out, "full validation test") {
		t.Errorf("expected description, got: %s", out)
	}
	if !strings.Contains(out, "mypack") {
		t.Errorf("expected pack name, got: %s", out)
	}
	if !strings.Contains(out, "mytool") {
		t.Errorf("expected inline tool name, got: %s", out)
	}
	if !strings.Contains(out, "Eligibility") {
		t.Errorf("expected eligibility, got: %s", out)
	}
	if !strings.Contains(out, "tool_calls") {
		t.Errorf("expected budget name, got: %s", out)
	}
	if !strings.Contains(out, "Rate limiting") {
		t.Errorf("expected rate limiting, got: %s", out)
	}
	if !strings.Contains(out, "Notifications") {
		t.Errorf("expected notifications, got: %s", out)
	}
}

// ─── formatJSON tests ───

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid", `{"key":"value"}`},
		{"invalid", `not json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatJSON([]byte(tt.input))
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}
