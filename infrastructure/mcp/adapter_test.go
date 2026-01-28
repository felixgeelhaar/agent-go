package mcp

import (
	"context"
	"encoding/json"
	"errors"
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

func TestMCPProxyTool_Annotations(t *testing.T) {
	t.Parallel()

	t.Run("returns set annotations", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "test_tool",
		}

		proxy := newMCPProxyTool(def, nil)
		// Set annotations after creation
		proxy.annot = tool.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
		}

		annot := proxy.Annotations()
		if !annot.ReadOnly {
			t.Error("ReadOnly should be true")
		}
		if annot.Destructive {
			t.Error("Destructive should be false")
		}
		if !annot.Idempotent {
			t.Error("Idempotent should be true")
		}
	})
}

func TestMCPProxyTool_Execute_Error(t *testing.T) {
	t.Parallel()

	t.Run("propagates caller error", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "failing_tool",
		}

		expectedErr := errors.New("tool execution failed")
		caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, expectedErr
		}

		proxy := newMCPProxyTool(def, caller)
		_, err := proxy.Execute(context.Background(), json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("Execute() should return error")
		}
		if err != expectedErr {
			t.Errorf("Execute() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name: "context_tool",
		}

		caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
			if ctx.Err() != nil {
				return tool.Result{}, ctx.Err()
			}
			return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
		}

		proxy := newMCPProxyTool(def, caller)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := proxy.Execute(ctx, json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("Execute() should return context error")
		}
		if err != context.Canceled {
			t.Errorf("Execute() error = %v, want context.Canceled", err)
		}
	})
}

func TestToolToMCPDef_WithAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("includes all tool properties", func(t *testing.T) {
		t.Parallel()

		schemaJSON := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"mode":{"type":"number"}}}`)
		mockT := &mockToolForAdapter{
			name:        "complex_tool",
			description: "A complex tool with schema and annotations",
			inputSchema: tool.NewSchema(schemaJSON),
			annotations: tool.Annotations{
				ReadOnly:    true,
				Destructive: false,
				Idempotent:  true,
			},
		}

		def := ToolToMCPDef(mockT)

		if def.Name != "complex_tool" {
			t.Errorf("Name = %s, want complex_tool", def.Name)
		}
		if def.Description != "A complex tool with schema and annotations" {
			t.Errorf("Description = %s, want 'A complex tool with schema and annotations'", def.Description)
		}
		if len(def.InputSchema) == 0 {
			t.Error("InputSchema should not be empty")
		}

		// Verify schema content
		var schemaMap map[string]interface{}
		if err := json.Unmarshal(def.InputSchema, &schemaMap); err != nil {
			t.Fatalf("Failed to unmarshal schema: %v", err)
		}
		if schemaMap["type"] != "object" {
			t.Errorf("Schema type = %v, want object", schemaMap["type"])
		}
	})
}

func TestMCPDefToTool_WithSchema(t *testing.T) {
	t.Parallel()

	t.Run("creates proxy tool with complex schema", func(t *testing.T) {
		t.Parallel()

		inputSchema := json.RawMessage(`{"type":"object","properties":{"file":{"type":"string"},"content":{"type":"string"}}}`)
		def := MCPToolDef{
			Name:        "write_file",
			Description: "Writes content to a file",
			InputSchema: inputSchema,
		}

		callCount := 0
		caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
			callCount++
			return tool.Result{Output: json.RawMessage(`{"success":true,"bytes_written":42}`)}, nil
		}

		proxyTool := MCPDefToTool(def, caller)

		// Verify schema is preserved
		schema := proxyTool.InputSchema()
		if schema.IsEmpty() {
			t.Error("InputSchema() should not be empty")
		}

		// Execute the tool
		result, err := proxyTool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt","content":"hello"}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if callCount != 1 {
			t.Errorf("Caller was called %d times, want 1", callCount)
		}

		// Verify output
		var output map[string]interface{}
		if err := json.Unmarshal(result.Output, &output); err != nil {
			t.Fatalf("Failed to unmarshal output: %v", err)
		}
		if output["success"] != true {
			t.Errorf("Output success = %v, want true", output["success"])
		}
	})
}

func TestNewMCPProxyTool_NilCaller(t *testing.T) {
	t.Parallel()

	t.Run("accepts nil caller", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name:        "nil_caller_tool",
			Description: "Tool with nil caller",
		}

		proxy := newMCPProxyTool(def, nil)
		if proxy == nil {
			t.Fatal("newMCPProxyTool() should not return nil")
		}

		if proxy.Name() != "nil_caller_tool" {
			t.Errorf("Name() = %s, want nil_caller_tool", proxy.Name())
		}

		// Note: Executing with nil caller will panic, which is expected behavior
		// We don't test execution in this case
	})
}
