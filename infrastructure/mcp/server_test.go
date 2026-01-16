package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/mcp"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// mockTool is a simple tool for testing.
type mockTool struct {
	name        string
	description string
	annotations tool.Annotations
	execute     func(ctx context.Context, input json.RawMessage) (tool.Result, error)
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return m.description }
func (m *mockTool) InputSchema() tool.Schema      { return tool.EmptySchema() }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.EmptySchema() }
func (m *mockTool) Annotations() tool.Annotations { return m.annotations }

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.execute != nil {
		return m.execute(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
}

func TestNewAgentServer(t *testing.T) {
	t.Parallel()

	t.Run("creates server with registry", func(t *testing.T) {
		t.Parallel()

		registry := memory.NewToolRegistry()
		mockT := &mockTool{
			name:        "test_tool",
			description: "A test tool",
		}
		registry.Register(mockT)

		srv := mcp.NewAgentServer(mcp.AgentServerConfig{
			Name:     "test-server",
			Version:  "1.0.0",
			Registry: registry,
		})

		if srv == nil {
			t.Fatal("NewAgentServer() returned nil")
		}

		if srv.Server() == nil {
			t.Error("Server() returned nil")
		}
	})

	t.Run("creates server without registry", func(t *testing.T) {
		t.Parallel()

		srv := mcp.NewAgentServer(mcp.AgentServerConfig{
			Name:    "test-server",
			Version: "1.0.0",
		})

		if srv == nil {
			t.Fatal("NewAgentServer() returned nil")
		}
	})

	t.Run("creates server with instructions", func(t *testing.T) {
		t.Parallel()

		srv := mcp.NewAgentServer(mcp.AgentServerConfig{
			Name:         "test-server",
			Version:      "1.0.0",
			Instructions: "Use this server for testing",
		})

		if srv == nil {
			t.Fatal("NewAgentServer() returned nil")
		}
	})
}

func TestAgentServer_AddTool(t *testing.T) {
	t.Parallel()

	t.Run("adds tool to server with registry", func(t *testing.T) {
		t.Parallel()

		registry := memory.NewToolRegistry()
		srv := mcp.NewAgentServer(mcp.AgentServerConfig{
			Name:     "test-server",
			Version:  "1.0.0",
			Registry: registry,
		})

		mockT := &mockTool{
			name:        "new_tool",
			description: "A new tool",
		}

		err := srv.AddTool(mockT)
		if err != nil {
			t.Fatalf("AddTool() error = %v", err)
		}

		// Verify tool was added to registry
		registeredTool, ok := registry.Get("new_tool")
		if !ok {
			t.Error("Tool was not added to registry")
		}

		if registeredTool.Name() != "new_tool" {
			t.Errorf("Tool name = %s, want new_tool", registeredTool.Name())
		}
	})

	t.Run("adds tool to server without registry", func(t *testing.T) {
		t.Parallel()

		srv := mcp.NewAgentServer(mcp.AgentServerConfig{
			Name:    "test-server",
			Version: "1.0.0",
		})

		mockT := &mockTool{
			name:        "new_tool",
			description: "A new tool",
		}

		// Should not error even without registry
		err := srv.AddTool(mockT)
		if err != nil {
			t.Fatalf("AddTool() error = %v", err)
		}
	})
}

func TestQuickServe(t *testing.T) {
	t.Parallel()

	// QuickServe is a blocking call, so we just test that it accepts the parameters
	// A full integration test would require mocking stdio
	registry := memory.NewToolRegistry()
	registry.Register(&mockTool{
		name:        "test_tool",
		description: "A test tool",
	})

	// Create a canceled context so it returns immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should return quickly due to canceled context
	err := mcp.QuickServe(ctx, "test", "1.0.0", registry)
	if err != nil && err != context.Canceled {
		t.Logf("QuickServe returned error (expected with cancelled context): %v", err)
	}
}

func TestToolToMCPDef(t *testing.T) {
	t.Parallel()

	mockT := &mockTool{
		name:        "test_tool",
		description: "A test tool description",
	}

	def := mcp.ToolToMCPDef(mockT)

	if def.Name != "test_tool" {
		t.Errorf("Name = %s, want test_tool", def.Name)
	}

	if def.Description != "A test tool description" {
		t.Errorf("Description = %s, want 'A test tool description'", def.Description)
	}
}

func TestMCPDefToTool(t *testing.T) {
	t.Parallel()

	def := mcp.MCPToolDef{
		Name:        "remote_tool",
		Description: "A remote tool",
	}

	called := false
	caller := func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
		called = true
		if name != "remote_tool" {
			t.Errorf("caller received name = %s, want remote_tool", name)
		}
		return tool.Result{Output: json.RawMessage(`{"result":"ok"}`)}, nil
	}

	proxyTool := mcp.MCPDefToTool(def, caller)

	if proxyTool.Name() != "remote_tool" {
		t.Errorf("Name = %s, want remote_tool", proxyTool.Name())
	}

	if proxyTool.Description() != "A remote tool" {
		t.Errorf("Description = %s, want 'A remote tool'", proxyTool.Description())
	}

	// Execute the tool
	result, err := proxyTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !called {
		t.Error("Caller was not invoked")
	}

	if string(result.Output) != `{"result":"ok"}` {
		t.Errorf("Output = %s, want {\"result\":\"ok\"}", result.Output)
	}
}

func TestAgentServer_Use(t *testing.T) {
	t.Parallel()

	srv := mcp.NewAgentServer(mcp.AgentServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
	})

	// Use should not panic when adding middlewares
	// We can't verify middleware is actually added without internals access
	// but we can verify the method doesn't panic
	srv.Use() // No-op with no middlewares
}

func TestAgentServer_ServeHTTP(t *testing.T) {
	t.Parallel()

	srv := mcp.NewAgentServer(mcp.AgentServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
	})

	// Create a canceled context to make ServeHTTP return quickly
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// ServeHTTP should return an error with canceled context
	err := srv.ServeHTTP(ctx, "localhost:0")
	// Either returns immediately with context error or http server error
	if err != nil && err != context.Canceled {
		t.Logf("ServeHTTP returned error (expected with canceled context): %v", err)
	}
}

func TestToolWithSchema(t *testing.T) {
	t.Parallel()

	// Test a tool with an actual schema
	schemaJSON := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)

	mockT := &mockToolWithSchema{
		name:        "schema_tool",
		description: "Tool with schema",
		inputSchema: schemaJSON,
	}

	registry := memory.NewToolRegistry()
	registry.Register(mockT)

	srv := mcp.NewAgentServer(mcp.AgentServerConfig{
		Name:     "test-server",
		Version:  "1.0.0",
		Registry: registry,
	})

	if srv == nil {
		t.Fatal("NewAgentServer() returned nil")
	}
}

// mockToolWithSchema implements tool.Tool with a schema
type mockToolWithSchema struct {
	name        string
	description string
	inputSchema json.RawMessage
}

func (m *mockToolWithSchema) Name() string        { return m.name }
func (m *mockToolWithSchema) Description() string { return m.description }
func (m *mockToolWithSchema) InputSchema() tool.Schema {
	if len(m.inputSchema) > 0 {
		return tool.NewSchema(m.inputSchema)
	}
	return tool.EmptySchema()
}
func (m *mockToolWithSchema) OutputSchema() tool.Schema     { return tool.EmptySchema() }
func (m *mockToolWithSchema) Annotations() tool.Annotations { return tool.Annotations{} }

func (m *mockToolWithSchema) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
}
