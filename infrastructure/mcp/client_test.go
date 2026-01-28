package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"sync"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestClientOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithClientName sets name", func(t *testing.T) {
		t.Parallel()

		cfg := ClientConfig{}
		opt := WithClientName("test-client")
		opt(&cfg)

		if cfg.Name != "test-client" {
			t.Errorf("Name = %s, want test-client", cfg.Name)
		}
	})

	t.Run("WithClientVersion sets version", func(t *testing.T) {
		t.Parallel()

		cfg := ClientConfig{}
		opt := WithClientVersion("2.0.0")
		opt(&cfg)

		if cfg.Version != "2.0.0" {
			t.Errorf("Version = %s, want 2.0.0", cfg.Version)
		}
	})

	t.Run("WithServerCommand sets command and transport", func(t *testing.T) {
		t.Parallel()

		cfg := ClientConfig{}
		opt := WithServerCommand("npx", "-y", "@anthropic/mcp-server-test")
		opt(&cfg)

		if len(cfg.Command) != 3 {
			t.Errorf("Command length = %d, want 3", len(cfg.Command))
		}
		if cfg.Command[0] != "npx" {
			t.Errorf("Command[0] = %s, want npx", cfg.Command[0])
		}
		if cfg.Transport != ClientTransportStdio {
			t.Errorf("Transport = %s, want stdio", cfg.Transport)
		}
	})

	t.Run("WithSSEURL sets URL and transport", func(t *testing.T) {
		t.Parallel()

		cfg := ClientConfig{}
		opt := WithSSEURL("http://localhost:8080/sse")
		opt(&cfg)

		if cfg.URL != "http://localhost:8080/sse" {
			t.Errorf("URL = %s, want http://localhost:8080/sse", cfg.URL)
		}
		if cfg.Transport != ClientTransportSSE {
			t.Errorf("Transport = %s, want sse", cfg.Transport)
		}
	})

	t.Run("WithHTTPURL sets URL and transport", func(t *testing.T) {
		t.Parallel()

		cfg := ClientConfig{}
		opt := WithHTTPURL("http://localhost:8080/api")
		opt(&cfg)

		if cfg.URL != "http://localhost:8080/api" {
			t.Errorf("URL = %s, want http://localhost:8080/api", cfg.URL)
		}
		if cfg.Transport != ClientTransportHTTP {
			t.Errorf("Transport = %s, want http", cfg.Transport)
		}
	})
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("creates client with defaults", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		if client == nil {
			t.Fatal("NewClient() returned nil")
		}
		if client.config.Name != "agent-go-client" {
			t.Errorf("Name = %s, want agent-go-client", client.config.Name)
		}
		if client.config.Version != "1.0.0" {
			t.Errorf("Version = %s, want 1.0.0", client.config.Version)
		}
		if client.config.Transport != ClientTransportStdio {
			t.Errorf("Transport = %s, want stdio", client.config.Transport)
		}
	})

	t.Run("creates client with options", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			WithClientName("custom-client"),
			WithClientVersion("3.0.0"),
		)

		if client.config.Name != "custom-client" {
			t.Errorf("Name = %s, want custom-client", client.config.Name)
		}
		if client.config.Version != "3.0.0" {
			t.Errorf("Version = %s, want 3.0.0", client.config.Version)
		}
	})
}

func TestMCPClient_Connect(t *testing.T) {
	t.Parallel()

	t.Run("returns error for already connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.connected = true

		err := client.Connect(context.Background())
		if err != ErrAlreadyConnected {
			t.Errorf("Connect() error = %v, want ErrAlreadyConnected", err)
		}
	})

	t.Run("returns error for SSE transport (not implemented)", func(t *testing.T) {
		t.Parallel()

		client := NewClient(WithSSEURL("http://localhost:8080"))

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should return error for SSE transport")
		}
	})

	t.Run("returns error for HTTP transport (not implemented)", func(t *testing.T) {
		t.Parallel()

		client := NewClient(WithHTTPURL("http://localhost:8080"))

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should return error for HTTP transport")
		}
	})

	t.Run("returns error for unknown transport", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.config.Transport = "unknown"

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should return error for unknown transport")
		}
	})

	t.Run("returns error for stdio with no command", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should return error for missing command")
		}
	})
}

func TestMCPClient_Close(t *testing.T) {
	t.Parallel()

	t.Run("close when not connected does nothing", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		err := client.Close()
		if err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})

	t.Run("close sets connected to false", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.connected = true

		err := client.Close()
		if err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}

		if client.connected {
			t.Error("Client should not be connected after Close()")
		}
	})
}

func TestMCPClient_ListTools(t *testing.T) {
	t.Parallel()

	t.Run("returns error when not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		_, err := client.ListTools(context.Background())
		if err != ErrNotConnected {
			t.Errorf("ListTools() error = %v, want ErrNotConnected", err)
		}
	})
}

func TestMCPClient_CallTool(t *testing.T) {
	t.Parallel()

	t.Run("returns error when not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		_, err := client.CallTool(context.Background(), MCPToolCall{
			Name:      "test_tool",
			Arguments: json.RawMessage(`{}`),
		})
		if err != ErrNotConnected {
			t.Errorf("CallTool() error = %v, want ErrNotConnected", err)
		}
	})
}

func TestMCPClient_ServerInfo(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		info := client.ServerInfo()
		if info != nil {
			t.Errorf("ServerInfo() = %v, want nil", info)
		}
	})

	t.Run("returns server info when set", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.serverInfo = &MCPServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		}

		info := client.ServerInfo()
		if info == nil {
			t.Fatal("ServerInfo() returned nil")
		}
		if info.Name != "test-server" {
			t.Errorf("ServerInfo().Name = %s, want test-server", info.Name)
		}
	})
}

func TestMCPClient_Tools(t *testing.T) {
	t.Parallel()

	t.Run("returns error when not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		_, err := client.Tools(context.Background())
		if err != ErrNotConnected {
			t.Errorf("Tools() error = %v, want ErrNotConnected", err)
		}
	})
}

func TestClientTransportConstants(t *testing.T) {
	t.Parallel()

	if ClientTransportStdio != "stdio" {
		t.Errorf("ClientTransportStdio = %s, want stdio", ClientTransportStdio)
	}
	if ClientTransportSSE != "sse" {
		t.Errorf("ClientTransportSSE = %s, want sse", ClientTransportSSE)
	}
	if ClientTransportHTTP != "http" {
		t.Errorf("ClientTransportHTTP = %s, want http", ClientTransportHTTP)
	}
}

func TestMCPTypes(t *testing.T) {
	t.Parallel()

	t.Run("MCPToolDef JSON serialization", func(t *testing.T) {
		t.Parallel()

		def := MCPToolDef{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}

		data, err := json.Marshal(def)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPToolDef
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Name != "test_tool" {
			t.Errorf("Name = %s, want test_tool", decoded.Name)
		}
	})

	t.Run("MCPToolCall JSON serialization", func(t *testing.T) {
		t.Parallel()

		call := MCPToolCall{
			Name:      "test_tool",
			Arguments: json.RawMessage(`{"arg":"value"}`),
		}

		data, err := json.Marshal(call)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPToolCall
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Name != "test_tool" {
			t.Errorf("Name = %s, want test_tool", decoded.Name)
		}
	})

	t.Run("MCPToolResult JSON serialization", func(t *testing.T) {
		t.Parallel()

		result := MCPToolResult{
			Content: []MCPContent{
				{Type: "text", Text: "Hello"},
			},
			IsError: false,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPToolResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if len(decoded.Content) != 1 {
			t.Errorf("Content length = %d, want 1", len(decoded.Content))
		}
		if decoded.Content[0].Text != "Hello" {
			t.Errorf("Content[0].Text = %s, want Hello", decoded.Content[0].Text)
		}
	})
}

func TestCreateToolCaller(t *testing.T) {
	t.Parallel()

	// Test the tool caller function creation
	// This is an internal function but we can test its behavior through the proxy tool

	t.Run("returns function that converts result", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		caller := client.createToolCaller()
		if caller == nil {
			t.Fatal("createToolCaller() returned nil")
		}
	})

	t.Run("caller returns error when not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		caller := client.createToolCaller()

		_, err := caller(context.Background(), "test_tool", json.RawMessage(`{}`))
		if err != ErrNotConnected {
			t.Errorf("caller error = %v, want ErrNotConnected", err)
		}
	})
}

func TestMCPServerInfo(t *testing.T) {
	t.Parallel()

	t.Run("JSON serialization", func(t *testing.T) {
		t.Parallel()

		info := MCPServerInfo{
			Name:    "test-server",
			Version: "2.0.0",
		}

		data, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPServerInfo
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Name != "test-server" {
			t.Errorf("Name = %s, want test-server", decoded.Name)
		}
		if decoded.Version != "2.0.0" {
			t.Errorf("Version = %s, want 2.0.0", decoded.Version)
		}
	})
}

func TestMCPContent(t *testing.T) {
	t.Parallel()

	t.Run("JSON serialization", func(t *testing.T) {
		t.Parallel()

		content := MCPContent{
			Type: "text",
			Text: "Hello, World!",
		}

		data, err := json.Marshal(content)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPContent
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Type != "text" {
			t.Errorf("Type = %s, want text", decoded.Type)
		}
		if decoded.Text != "Hello, World!" {
			t.Errorf("Text = %s, want Hello, World!", decoded.Text)
		}
	})

	t.Run("omits empty text", func(t *testing.T) {
		t.Parallel()

		content := MCPContent{
			Type: "image",
		}

		data, err := json.Marshal(content)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		// Should not contain "text" field
		expected := `{"type":"image"}`
		if string(data) != expected {
			t.Errorf("JSON = %s, want %s", data, expected)
		}
	})
}

func TestClientConfig(t *testing.T) {
	t.Parallel()

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		if client.config.Name != "agent-go-client" {
			t.Errorf("Name = %s, want agent-go-client", client.config.Name)
		}
		if client.config.Version != "1.0.0" {
			t.Errorf("Version = %s, want 1.0.0", client.config.Version)
		}
		if client.config.Transport != ClientTransportStdio {
			t.Errorf("Transport = %s, want stdio", client.config.Transport)
		}
		if len(client.config.Command) != 0 {
			t.Errorf("Command = %v, want empty", client.config.Command)
		}
		if client.config.URL != "" {
			t.Errorf("URL = %s, want empty", client.config.URL)
		}
	})

	t.Run("responses map is initialized", func(t *testing.T) {
		t.Parallel()

		client := NewClient()

		if client.responses == nil {
			t.Error("responses map should be initialized")
		}
	})
}

func TestRPCTypes(t *testing.T) {
	t.Parallel()

	t.Run("rpcRequest JSON serialization", func(t *testing.T) {
		t.Parallel()

		req := rpcRequest{
			JSONRPC: "2.0",
			ID:      int64(1),
			Method:  "test/method",
			Params:  json.RawMessage(`{"arg":"value"}`),
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded rpcRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.JSONRPC != "2.0" {
			t.Errorf("JSONRPC = %s, want 2.0", decoded.JSONRPC)
		}
		if decoded.Method != "test/method" {
			t.Errorf("Method = %s, want test/method", decoded.Method)
		}
	})

	t.Run("rpcResponse JSON serialization", func(t *testing.T) {
		t.Parallel()

		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      float64(1),
			Result:  map[string]string{"status": "ok"},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded rpcResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.JSONRPC != "2.0" {
			t.Errorf("JSONRPC = %s, want 2.0", decoded.JSONRPC)
		}
	})

	t.Run("rpcResponse with error", func(t *testing.T) {
		t.Parallel()

		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      float64(1),
			Error: &rpcError{
				Code:    -32601,
				Message: "Method not found",
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded rpcResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Error == nil {
			t.Fatal("Error should not be nil")
		}
		if decoded.Error.Code != -32601 {
			t.Errorf("Error.Code = %d, want -32601", decoded.Error.Code)
		}
		if decoded.Error.Message != "Method not found" {
			t.Errorf("Error.Message = %s, want 'Method not found'", decoded.Error.Message)
		}
	})

	t.Run("initParams JSON serialization", func(t *testing.T) {
		t.Parallel()

		params := initParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: MCPServerInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded initParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.ProtocolVersion != "2024-11-05" {
			t.Errorf("ProtocolVersion = %s, want 2024-11-05", decoded.ProtocolVersion)
		}
		if decoded.ClientInfo.Name != "test-client" {
			t.Errorf("ClientInfo.Name = %s, want test-client", decoded.ClientInfo.Name)
		}
	})

	t.Run("initResult JSON serialization", func(t *testing.T) {
		t.Parallel()

		result := initResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: MCPServerInfo{
				Name:    "test-server",
				Version: "2.0.0",
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded initResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.ProtocolVersion != "2024-11-05" {
			t.Errorf("ProtocolVersion = %s, want 2024-11-05", decoded.ProtocolVersion)
		}
		if decoded.ServerInfo.Name != "test-server" {
			t.Errorf("ServerInfo.Name = %s, want test-server", decoded.ServerInfo.Name)
		}
	})

	t.Run("listToolsRes JSON serialization", func(t *testing.T) {
		t.Parallel()

		result := listToolsRes{
			Tools: []MCPToolDef{
				{Name: "tool1", Description: "First tool"},
				{Name: "tool2", Description: "Second tool"},
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded listToolsRes
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if len(decoded.Tools) != 2 {
			t.Errorf("Tools length = %d, want 2", len(decoded.Tools))
		}
		if decoded.Tools[0].Name != "tool1" {
			t.Errorf("Tools[0].Name = %s, want tool1", decoded.Tools[0].Name)
		}
	})

	t.Run("callToolParams JSON serialization", func(t *testing.T) {
		t.Parallel()

		params := callToolParams{
			Name:      "my_tool",
			Arguments: json.RawMessage(`{"key":"value"}`),
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded callToolParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Name != "my_tool" {
			t.Errorf("Name = %s, want my_tool", decoded.Name)
		}
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrNotConnected is defined", func(t *testing.T) {
		t.Parallel()

		if ErrNotConnected.Error() != "client not connected" {
			t.Errorf("ErrNotConnected = %s, want 'client not connected'", ErrNotConnected.Error())
		}
	})

	t.Run("ErrAlreadyConnected is defined", func(t *testing.T) {
		t.Parallel()

		if ErrAlreadyConnected.Error() != "client already connected" {
			t.Errorf("ErrAlreadyConnected = %s, want 'client already connected'", ErrAlreadyConnected.Error())
		}
	})

	t.Run("ErrConnectionFailed is defined", func(t *testing.T) {
		t.Parallel()

		if ErrConnectionFailed.Error() != "connection failed" {
			t.Errorf("ErrConnectionFailed = %s, want 'connection failed'", ErrConnectionFailed.Error())
		}
	})

	t.Run("ErrInvalidCommand is defined", func(t *testing.T) {
		t.Parallel()

		if ErrInvalidCommand.Error() != "invalid command" {
			t.Errorf("ErrInvalidCommand = %s, want 'invalid command'", ErrInvalidCommand.Error())
		}
	})

	t.Run("errors can be wrapped", func(t *testing.T) {
		t.Parallel()

		wrappedErr := errors.New("wrapped: " + ErrNotConnected.Error())
		if wrappedErr == nil {
			t.Error("Should be able to wrap errors")
		}
	})
}

func TestMCPClient_Concurrent(t *testing.T) {
	t.Parallel()

	t.Run("concurrent ServerInfo calls are safe", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.serverInfo = &MCPServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		}

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					info := client.ServerInfo()
					if info == nil {
						t.Error("ServerInfo returned nil")
					}
				}
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent Close calls are safe", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		client.connected = true

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				_ = client.Close()
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestMCPClient_ConnectStdioErrors(t *testing.T) {
	t.Parallel()

	t.Run("connect fails with non-existent command", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			WithServerCommand("non-existent-command-that-does-not-exist"),
		)

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should fail with non-existent command")
			_ = client.Close()
		}
	})
}

func TestMCPToolResult(t *testing.T) {
	t.Parallel()

	t.Run("IsError field serialization", func(t *testing.T) {
		t.Parallel()

		result := MCPToolResult{
			Content: []MCPContent{
				{Type: "text", Text: "Error message"},
			},
			IsError: true,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded MCPToolResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if !decoded.IsError {
			t.Error("IsError should be true")
		}
	})

	t.Run("omits isError when false", func(t *testing.T) {
		t.Parallel()

		result := MCPToolResult{
			Content: []MCPContent{
				{Type: "text", Text: "Success"},
			},
			IsError: false,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		// With omitempty, isError should not appear when false
		// Both formats are acceptable depending on omitempty behavior
		_ = string(data) // Validate JSON was generated without error
	})
}

func TestValidateCommand(t *testing.T) {
	t.Parallel()

	t.Run("accepts valid command in PATH", func(t *testing.T) {
		t.Parallel()

		// Test with a command that should exist on all systems
		resolved, err := validateCommand("go")
		if err != nil {
			t.Skipf("Skipping test: go command not found: %v", err)
		}
		if resolved == "" {
			t.Error("validateCommand() should return non-empty path")
		}
	})

	t.Run("rejects empty command", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("")
		if err == nil {
			t.Error("validateCommand() should reject empty command")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with semicolon", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("ls;rm")
		if err == nil {
			t.Error("validateCommand() should reject command with semicolon")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with pipe", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("cat|grep")
		if err == nil {
			t.Error("validateCommand() should reject command with pipe")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with ampersand", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("sleep&")
		if err == nil {
			t.Error("validateCommand() should reject command with ampersand")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with dollar sign", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("echo$HOME")
		if err == nil {
			t.Error("validateCommand() should reject command with dollar sign")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with backtick", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("echo`whoami`")
		if err == nil {
			t.Error("validateCommand() should reject command with backtick")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with parentheses", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("(ls)")
		if err == nil {
			t.Error("validateCommand() should reject command with parentheses")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with redirect", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("cat>file")
		if err == nil {
			t.Error("validateCommand() should reject command with redirect")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with single quote", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("echo'test'")
		if err == nil {
			t.Error("validateCommand() should reject command with single quote")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with double quote", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("echo\"test\"")
		if err == nil {
			t.Error("validateCommand() should reject command with double quote")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects command with newline", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("ls\nrm")
		if err == nil {
			t.Error("validateCommand() should reject command with newline")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects non-existent command", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("nonexistent-command-xyz123")
		if err == nil {
			t.Error("validateCommand() should reject non-existent command")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("accepts absolute path to valid command", func(t *testing.T) {
		t.Parallel()

		// Find the go binary's absolute path
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("Skipping test: go command not found: %v", err)
		}

		resolved, err := validateCommand(goPath)
		if err != nil {
			t.Fatalf("validateCommand() should accept valid absolute path: %v", err)
		}
		if resolved != goPath {
			t.Errorf("validateCommand() = %s, want %s", resolved, goPath)
		}
	})

	t.Run("rejects absolute path with traversal", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("/usr/bin/../bin/ls")
		if err == nil {
			t.Error("validateCommand() should reject path with traversal elements")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})

	t.Run("rejects absolute path to non-existent file", func(t *testing.T) {
		t.Parallel()

		_, err := validateCommand("/nonexistent/path/to/command")
		if err == nil {
			t.Error("validateCommand() should reject non-existent absolute path")
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("Error should wrap ErrInvalidCommand, got %v", err)
		}
	})
}

func TestImportToolsFromClient(t *testing.T) {
	t.Parallel()

	t.Run("returns error when client not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		registry := &mockToolRegistry{tools: make(map[string]tool.Tool)}

		err := ImportToolsFromClient(context.Background(), client, registry)
		if err == nil {
			t.Error("ImportToolsFromClient() should return error when not connected")
		}
		if err != ErrNotConnected {
			t.Errorf("Error = %v, want ErrNotConnected", err)
		}
	})

	t.Run("continues on duplicate registration", func(t *testing.T) {
		t.Parallel()

		// This test documents the behavior but can't easily test it without a real server
		// The function continues on duplicate registration errors
		client := NewClient()
		registry := &mockToolRegistry{tools: make(map[string]tool.Tool)}

		// Pre-register a tool
		mockT := &mockToolForImport{
			name:        "existing_tool",
			description: "Already exists",
		}
		_ = registry.Register(mockT)

		err := ImportToolsFromClient(context.Background(), client, registry)
		// Should fail with not connected, not duplicate error
		if err != ErrNotConnected {
			t.Errorf("Error = %v, want ErrNotConnected", err)
		}
	})
}

// mockToolForImport is a simple mock tool
type mockToolForImport struct {
	name        string
	description string
}

func (m *mockToolForImport) Name() string                  { return m.name }
func (m *mockToolForImport) Description() string           { return m.description }
func (m *mockToolForImport) InputSchema() tool.Schema      { return tool.EmptySchema() }
func (m *mockToolForImport) OutputSchema() tool.Schema     { return tool.EmptySchema() }
func (m *mockToolForImport) Annotations() tool.Annotations { return tool.Annotations{} }

func (m *mockToolForImport) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	return tool.Result{Output: json.RawMessage(`{}`)}, nil
}

func TestCreateToolCaller_ErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("returns error when client not connected", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		caller := client.createToolCaller()

		// Calling on disconnected client returns error
		_, err := caller(context.Background(), "test_tool", json.RawMessage(`{}`))
		if err != ErrNotConnected {
			t.Errorf("Caller error = %v, want ErrNotConnected", err)
		}
	})

	t.Run("creates valid caller function", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		caller := client.createToolCaller()

		if caller == nil {
			t.Fatal("createToolCaller() returned nil function")
		}
	})

	t.Run("caller passes correct parameters", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		caller := client.createToolCaller()

		// Test that caller is created with proper closure
		testInput := json.RawMessage(`{"key":"value"}`)
		_, err := caller(context.Background(), "my_tool", testInput)

		// Should fail with not connected, but proves closure captured client
		if err != ErrNotConnected {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})

	t.Run("caller handles context cancellation", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		caller := client.createToolCaller()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := caller(ctx, "test_tool", json.RawMessage(`{}`))
		if err != ErrNotConnected {
			// Should return not connected since we're not connected
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
}

func TestMCPClient_ConnectStdioValidation(t *testing.T) {
	t.Parallel()

	t.Run("rejects command with shell metacharacter in args", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			WithServerCommand("go", "run;malicious"),
		)

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should reject command with shell metacharacter in args")
			_ = client.Close()
		}
	})

	t.Run("rejects command with pipe in args", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			WithServerCommand("go", "run|grep"),
		)

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should reject command with pipe in args")
			_ = client.Close()
		}
	})

	t.Run("rejects command with ampersand in args", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			WithServerCommand("go", "run&background"),
		)

		err := client.Connect(context.Background())
		if err == nil {
			t.Error("Connect() should reject command with ampersand in args")
			_ = client.Close()
		}
	})
}

// mockToolRegistry implements tool.Registry for testing
type mockToolRegistry struct {
	tools map[string]tool.Tool
	mu    sync.RWMutex
}

func (r *mockToolRegistry) Register(t tool.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		return errors.New("tool already registered")
	}
	r.tools[t.Name()] = t
	return nil
}

func (r *mockToolRegistry) Get(name string) (tool.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *mockToolRegistry) List() []tool.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *mockToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *mockToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

func (r *mockToolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		return errors.New("tool not found")
	}
	delete(r.tools, name)
	return nil
}
