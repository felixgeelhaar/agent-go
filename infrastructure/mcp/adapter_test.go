package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestMCPProxyTool(t *testing.T) {
	t.Parallel()

	t.Run("Name returns def name", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name:        "test_tool",
			Description: "A test tool",
		}

		proxy := newMCPProxyTool(def, nil)
		if proxy.Name() != "test_tool" {
			t.Errorf("Name() = %s, want test_tool", proxy.Name())
		}
	})

	t.Run("Description returns def description", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name:        "test_tool",
			Description: "A test tool description",
		}

		proxy := newMCPProxyTool(def, nil)
		if proxy.Description() != "A test tool description" {
			t.Errorf("Description() = %s, want 'A test tool description'", proxy.Description())
		}
	})

	t.Run("InputSchema returns empty schema when no input schema", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "test_tool",
		}

		proxy := newMCPProxyTool(def, nil)
		schema := proxy.InputSchema()
		if !schema.IsEmpty() {
			t.Error("InputSchema() should be empty when no input schema defined")
		}
	})

	t.Run("InputSchema returns schema when defined", func(t *testing.T) {
		t.Parallel()

		inputSchema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
		def := MCPToolDef{
			Name:        "test_tool",
			InputSchema: inputSchema,
		}

		proxy := newMCPProxyTool(def, nil)
		schema := proxy.InputSchema()
		if schema.IsEmpty() {
			t.Error("InputSchema() should not be empty when input schema defined")
		}
	})

	t.Run("OutputSchema returns empty schema", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "test_tool",
		}

		proxy := newMCPProxyTool(def, nil)
		schema := proxy.OutputSchema()
		if !schema.IsEmpty() {
			t.Error("OutputSchema() should be empty")
		}
	})

	t.Run("Annotations returns default annotations", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "test_tool",
		}

		proxy := newMCPProxyTool(def, nil)
		annot := proxy.Annotations()
		// Default annotations should have zero values
		if annot.ReadOnly {
			t.Error("Default Annotations.ReadOnly should be false")
		}
		if annot.Destructive {
			t.Error("Default Annotations.Destructive should be false")
		}
	})

	t.Run("Execute calls the caller function", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "test_tool",
		}

		called := false
		caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
			called = true
			if name != "test_tool" {
				t.Errorf("caller received name = %s, want test_tool", name)
			}
			return tool.Result{Output: json.RawMessage(`{"result":"success"}`)}, nil
		}

		proxy := newMCPProxyTool(def, caller)
		result, err := proxy.Execute(context.Background(), json.RawMessage(`{"arg":"value"}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !called {
			t.Error("Caller was not invoked")
		}

		if string(result.Output) != `{"result":"success"}` {
			t.Errorf("Output = %s, want {\"result\":\"success\"}", result.Output)
		}
	})
}

func TestToolToMCPDef(t *testing.T) {
	t.Parallel()

	t.Run("converts tool with empty schema", func(t *testing.T) {
		t.Parallel()

		mockT := &mockToolForAdapter{
			name:        "empty_schema_tool",
			description: "Tool with empty schema",
			inputSchema: tool.EmptySchema(),
		}

		def := ToolToMCPDef(mockT)

		if def.Name != "empty_schema_tool" {
			t.Errorf("Name = %s, want empty_schema_tool", def.Name)
		}
		if def.Description != "Tool with empty schema" {
			t.Errorf("Description = %s, want 'Tool with empty schema'", def.Description)
		}
		if len(def.InputSchema) != 0 {
			t.Errorf("InputSchema should be empty, got %s", def.InputSchema)
		}
	})

	t.Run("converts tool with input schema", func(t *testing.T) {
		t.Parallel()

		schemaJSON := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
		mockT := &mockToolForAdapter{
			name:        "schema_tool",
			description: "Tool with schema",
			inputSchema: tool.NewSchema(schemaJSON),
		}

		def := ToolToMCPDef(mockT)

		if def.Name != "schema_tool" {
			t.Errorf("Name = %s, want schema_tool", def.Name)
		}
		if len(def.InputSchema) == 0 {
			t.Error("InputSchema should not be empty")
		}
	})
}

func TestMCPDefToTool(t *testing.T) {
	t.Parallel()

	t.Run("creates proxy tool from definition", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name:        "remote_tool",
			Description: "A remote tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}

		caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		proxyTool := MCPDefToTool(def, caller)

		if proxyTool.Name() != "remote_tool" {
			t.Errorf("Name() = %s, want remote_tool", proxyTool.Name())
		}
		if proxyTool.Description() != "A remote tool" {
			t.Errorf("Description() = %s, want 'A remote tool'", proxyTool.Description())
		}
	})
}

// mockToolForAdapter is a mock tool for testing adapter functions
type mockToolForAdapter struct {
	name        string
	description string
	inputSchema tool.Schema
	annotations tool.Annotations
}

func (m *mockToolForAdapter) Name() string                  { return m.name }
func (m *mockToolForAdapter) Description() string           { return m.description }
func (m *mockToolForAdapter) InputSchema() tool.Schema      { return m.inputSchema }
func (m *mockToolForAdapter) OutputSchema() tool.Schema     { return tool.EmptySchema() }
func (m *mockToolForAdapter) Annotations() tool.Annotations { return m.annotations }

func (m *mockToolForAdapter) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	return tool.Result{Output: json.RawMessage(`{}`)}, nil
}
