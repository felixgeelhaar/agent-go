package config

import (
	"encoding/json"
	"testing"
)

func TestGenerateSchema(t *testing.T) {
	schema := GenerateSchema()

	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("Schema = %s, want draft/2020-12", schema.Schema)
	}
	if schema.Type != "object" {
		t.Errorf("Type = %s, want object", schema.Type)
	}
	if schema.Title != "Agent Configuration" {
		t.Errorf("Title = %s, want Agent Configuration", schema.Title)
	}

	// Check required fields
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}
	if !requiredSet["name"] {
		t.Error("name should be required")
	}
	if !requiredSet["version"] {
		t.Error("version should be required")
	}

	// Check top-level properties
	expectedProps := []string{"name", "version", "description", "agent", "tools", "policy", "resilience", "notification", "variables"}
	for _, prop := range expectedProps {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("missing property: %s", prop)
		}
	}
}

func TestGenerateSchema_AgentProperties(t *testing.T) {
	schema := GenerateSchema()
	agent := schema.Properties["agent"]

	if agent.Type != "object" {
		t.Errorf("agent.Type = %s, want object", agent.Type)
	}

	expectedProps := []string{"max_steps", "default_goal", "initial_state"}
	for _, prop := range expectedProps {
		if _, ok := agent.Properties[prop]; !ok {
			t.Errorf("agent missing property: %s", prop)
		}
	}

	// Check initial_state enum
	initialState := agent.Properties["initial_state"]
	if len(initialState.Enum) != 5 {
		t.Errorf("initial_state.Enum has %d values, want 5", len(initialState.Enum))
	}
}

func TestGenerateSchema_ToolsProperties(t *testing.T) {
	schema := GenerateSchema()
	tools := schema.Properties["tools"]

	if tools.Type != "object" {
		t.Errorf("tools.Type = %s, want object", tools.Type)
	}

	expectedProps := []string{"packs", "inline", "eligibility"}
	for _, prop := range expectedProps {
		if _, ok := tools.Properties[prop]; !ok {
			t.Errorf("tools missing property: %s", prop)
		}
	}

	// Check packs is array
	packs := tools.Properties["packs"]
	if packs.Type != "array" {
		t.Errorf("packs.Type = %s, want array", packs.Type)
	}
}

func TestGenerateSchema_PolicyProperties(t *testing.T) {
	schema := GenerateSchema()
	policy := schema.Properties["policy"]

	if policy.Type != "object" {
		t.Errorf("policy.Type = %s, want object", policy.Type)
	}

	expectedProps := []string{"budgets", "approval", "transitions", "rate_limit"}
	for _, prop := range expectedProps {
		if _, ok := policy.Properties[prop]; !ok {
			t.Errorf("policy missing property: %s", prop)
		}
	}
}

func TestGenerateSchema_ResilienceProperties(t *testing.T) {
	schema := GenerateSchema()
	resilience := schema.Properties["resilience"]

	if resilience.Type != "object" {
		t.Errorf("resilience.Type = %s, want object", resilience.Type)
	}

	expectedProps := []string{"timeout", "retry", "circuit_breaker", "bulkhead"}
	for _, prop := range expectedProps {
		if _, ok := resilience.Properties[prop]; !ok {
			t.Errorf("resilience missing property: %s", prop)
		}
	}
}

func TestGenerateSchema_NotificationProperties(t *testing.T) {
	schema := GenerateSchema()
	notification := schema.Properties["notification"]

	if notification.Type != "object" {
		t.Errorf("notification.Type = %s, want object", notification.Type)
	}

	expectedProps := []string{"enabled", "endpoints", "batching", "event_filter"}
	for _, prop := range expectedProps {
		if _, ok := notification.Properties[prop]; !ok {
			t.Errorf("notification missing property: %s", prop)
		}
	}
}

func TestSchemaJSON(t *testing.T) {
	jsonStr, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("SchemaJSON() returned invalid JSON: %v", err)
	}

	// Check some key fields
	if parsed["$schema"] == nil {
		t.Error("Schema missing $schema")
	}
	if parsed["title"] != "Agent Configuration" {
		t.Errorf("title = %v, want Agent Configuration", parsed["title"])
	}
	if parsed["type"] != "object" {
		t.Errorf("type = %v, want object", parsed["type"])
	}
}

func TestSchemaJSON_ValidFormat(t *testing.T) {
	jsonStr, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON() error = %v", err)
	}

	// The output should be indented
	if len(jsonStr) > 0 && jsonStr[0] != '{' {
		t.Error("SchemaJSON() should start with {")
	}

	// Should contain newlines (indented format)
	if !contains(jsonStr, "\n") {
		t.Error("SchemaJSON() should be indented (contain newlines)")
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
