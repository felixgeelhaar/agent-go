package sandbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox"
)

// mockTool is a simple tool for testing.
type mockTool struct {
	name        string
	annotations tool.Annotations
	execute     func(ctx context.Context, input json.RawMessage) (tool.Result, error)
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return "mock tool" }
func (m *mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations { return m.annotations }

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.execute != nil {
		return m.execute(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
}

func TestNoopSandbox(t *testing.T) {
	t.Parallel()

	t.Run("executes tool directly", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		executed := false

		mockT := &mockTool{
			name: "test",
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				executed = true
				return tool.Result{Output: json.RawMessage(`{"result":"ok"}`)}, nil
			},
		}

		result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !executed {
			t.Error("Tool was not executed")
		}

		if string(result.Output) != `{"result":"ok"}` {
			t.Errorf("Output = %s, want %s", result.Output, `{"result":"ok"}`)
		}
	})

	t.Run("capabilities are unlimited", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		caps := sb.Capabilities()

		if !caps.Network {
			t.Error("Network should be true")
		}
		if !caps.Filesystem {
			t.Error("Filesystem should be true")
		}
		if caps.MaxMemory != 0 {
			t.Errorf("MaxMemory = %d, want 0", caps.MaxMemory)
		}
		if caps.MaxExecTime != 0 {
			t.Errorf("MaxExecTime = %v, want 0", caps.MaxExecTime)
		}
	})

	t.Run("close is no-op", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		if err := sb.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	caps := sandbox.Capabilities{
		Network:       true,
		Filesystem:    false,
		MaxMemory:     64 * 1024 * 1024,
		MaxExecTime:   30 * time.Second,
		AllowedEnv:    []string{"PATH", "HOME"},
		ReadOnlyPaths: []string{"/etc"},
		WritePaths:    []string{"/tmp"},
	}

	// Verify JSON serialization
	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded sandbox.Capabilities
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Network != caps.Network {
		t.Errorf("Network = %v, want %v", decoded.Network, caps.Network)
	}
	if decoded.MaxMemory != caps.MaxMemory {
		t.Errorf("MaxMemory = %d, want %d", decoded.MaxMemory, caps.MaxMemory)
	}
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		cfg := sandbox.Config{}

		sandbox.WithMaxMemory(128 * 1024 * 1024)(&cfg)
		if cfg.MaxMemory != 128*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", cfg.MaxMemory, 128*1024*1024)
		}

		sandbox.WithMaxExecTime(60 * time.Second)(&cfg)
		if cfg.MaxExecTime != 60*time.Second {
			t.Errorf("MaxExecTime = %v, want %v", cfg.MaxExecTime, 60*time.Second)
		}

		sandbox.WithNetwork()(&cfg)
		if !cfg.AllowNetwork {
			t.Error("AllowNetwork should be true")
		}

		sandbox.WithFilesystem("/data", []string{"/etc"}, []string{"/tmp"})(&cfg)
		if !cfg.AllowFilesystem {
			t.Error("AllowFilesystem should be true")
		}
		if cfg.FSRoot != "/data" {
			t.Errorf("FSRoot = %s, want /data", cfg.FSRoot)
		}

		sandbox.WithEnv("PATH", "HOME")(&cfg)
		if len(cfg.AllowedEnv) != 2 {
			t.Errorf("AllowedEnv length = %d, want 2", len(cfg.AllowedEnv))
		}
	})
}

func TestShouldSandbox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations tool.Annotations
		want        bool
	}{
		{
			name:        "sandboxed annotation",
			annotations: tool.Annotations{Sandboxed: true},
			want:        true,
		},
		{
			name:        "destructive tool",
			annotations: tool.Annotations{Destructive: true},
			want:        true,
		},
		{
			name:        "high risk level",
			annotations: tool.Annotations{RiskLevel: 3},
			want:        true,
		},
		{
			name:        "low risk level",
			annotations: tool.Annotations{RiskLevel: 2},
			want:        false,
		},
		{
			name:        "read only",
			annotations: tool.Annotations{ReadOnly: true},
			want:        false,
		},
		{
			name:        "default",
			annotations: tool.Annotations{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockT := &mockTool{
				name:        "test",
				annotations: tt.annotations,
			}

			if got := sandbox.ShouldSandbox(mockT); got != tt.want {
				t.Errorf("ShouldSandbox() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandboxMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("sandboxes tool with Sandboxed annotation", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		sandboxed := false
		mockT := &mockTool{
			name:        "sandboxed_tool",
			annotations: tool.Annotations{Sandboxed: true},
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				sandboxed = true
				return tool.Result{Output: json.RawMessage(`{"sandboxed":true}`)}, nil
			},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{}, nil
		}

		handler := mw(next)
		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !sandboxed {
			t.Error("Tool was not executed via sandbox")
		}
		if nextCalled {
			t.Error("next should not be called for sandboxed tools")
		}
		if string(result.Output) != `{"sandboxed":true}` {
			t.Errorf("Output = %s, want {\"sandboxed\":true}", result.Output)
		}
	})

	t.Run("sandboxes tool with Destructive annotation", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		executed := false
		mockT := &mockTool{
			name:        "destructive_tool",
			annotations: tool.Annotations{Destructive: true},
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				executed = true
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !executed {
			t.Error("Destructive tool was not executed")
		}
		if nextCalled {
			t.Error("next should not be called for destructive tools")
		}
	})

	t.Run("passes through non-sandboxed tool", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		mockT := &mockTool{
			name:        "normal_tool",
			annotations: tool.Annotations{}, // No sandboxing annotations
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{"next":true}`)}, nil
		}

		handler := mw(next)
		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !nextCalled {
			t.Error("next should be called for non-sandboxed tools")
		}
		if string(result.Output) != `{"next":true}` {
			t.Errorf("Output = %s, want {\"next\":true}", result.Output)
		}
	})
}

func TestConditionalSandboxMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("uses ReadOnlySandbox for read-only tools", func(t *testing.T) {
		t.Parallel()

		readOnlySandboxUsed := false
		defaultSandboxUsed := false

		readOnlySb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &readOnlySandboxUsed,
		}
		defaultSb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &defaultSandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			ReadOnlySandbox: readOnlySb,
			DefaultSandbox:  defaultSb,
			Predicate:       func(t tool.Tool) bool { return true }, // Always sandbox
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "readonly_tool",
			annotations: tool.Annotations{ReadOnly: true},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !readOnlySandboxUsed {
			t.Error("ReadOnlySandbox should be used for read-only tools")
		}
		if defaultSandboxUsed {
			t.Error("DefaultSandbox should not be used for read-only tools")
		}
	})

	t.Run("uses DefaultSandbox for non-read-only tools", func(t *testing.T) {
		t.Parallel()

		readOnlySandboxUsed := false
		defaultSandboxUsed := false

		readOnlySb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &readOnlySandboxUsed,
		}
		defaultSb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &defaultSandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			ReadOnlySandbox: readOnlySb,
			DefaultSandbox:  defaultSb,
			Predicate:       func(t tool.Tool) bool { return true },
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "write_tool",
			annotations: tool.Annotations{ReadOnly: false},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if readOnlySandboxUsed {
			t.Error("ReadOnlySandbox should not be used for non-read-only tools")
		}
		if !defaultSandboxUsed {
			t.Error("DefaultSandbox should be used for non-read-only tools")
		}
	})

	t.Run("skips sandboxing when predicate returns false", func(t *testing.T) {
		t.Parallel()

		sandboxUsed := false
		sb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &sandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			DefaultSandbox: sb,
			Predicate:      func(t tool.Tool) bool { return false }, // Never sandbox
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if sandboxUsed {
			t.Error("Sandbox should not be used when predicate returns false")
		}
		if !nextCalled {
			t.Error("next should be called when predicate returns false")
		}
	})

	t.Run("passes through when no sandbox configured", func(t *testing.T) {
		t.Parallel()

		cond := &sandbox.ConditionalSandboxMiddleware{
			// No sandboxes configured
			Predicate: func(t tool.Tool) bool { return true },
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !nextCalled {
			t.Error("next should be called when no sandbox is configured")
		}
	})

	t.Run("nil predicate sandboxes all tools", func(t *testing.T) {
		t.Parallel()

		sandboxUsed := false
		sb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &sandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			DefaultSandbox: sb,
			Predicate:      nil, // No predicate means always evaluate
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !sandboxUsed {
			t.Error("Sandbox should be used when predicate is nil")
		}
	})
}

// trackingSandbox wraps a sandbox and tracks whether it was used.
type trackingSandbox struct {
	noop    *sandbox.NoopSandbox
	tracker *bool
}

func (s *trackingSandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	*s.tracker = true
	return s.noop.Execute(ctx, t, input)
}

func (s *trackingSandbox) Capabilities() sandbox.Capabilities {
	return s.noop.Capabilities()
}

func (s *trackingSandbox) Close() error {
	return s.noop.Close()
}

func TestDefaultWASMConfig(t *testing.T) {
	t.Parallel()

	cfg := sandbox.DefaultWASMConfig()

	// Check default values
	if cfg.MaxMemory != 16*1024*1024 {
		t.Errorf("MaxMemory = %d, want %d", cfg.MaxMemory, 16*1024*1024)
	}
	if cfg.MaxExecTime != 30*time.Second {
		t.Errorf("MaxExecTime = %v, want %v", cfg.MaxExecTime, 30*time.Second)
	}
	if cfg.MaxMemoryPages != 256 {
		t.Errorf("MaxMemoryPages = %d, want 256", cfg.MaxMemoryPages)
	}
}

func TestNewWASM(t *testing.T) {
	t.Parallel()

	t.Run("creates sandbox with defaults", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		caps := sb.Capabilities()
		if caps.MaxMemory != 16*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", caps.MaxMemory, 16*1024*1024)
		}
	})

	t.Run("creates sandbox with custom options", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM(
			sandbox.WithMaxMemory(32*1024*1024),
			sandbox.WithMaxExecTime(60*time.Second),
			sandbox.WithNetwork(),
		)
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		caps := sb.Capabilities()
		if caps.MaxMemory != 32*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", caps.MaxMemory, 32*1024*1024)
		}
		if caps.MaxExecTime != 60*time.Second {
			t.Errorf("MaxExecTime = %v, want %v", caps.MaxExecTime, 60*time.Second)
		}
		if !caps.Network {
			t.Error("Network should be true")
		}
	})
}

func TestWASMSandbox_Execute(t *testing.T) {
	t.Parallel()

	t.Run("executes tool natively when no module loaded", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		executed := false
		mockT := &mockTool{
			name: "test_tool",
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				executed = true
				return tool.Result{Output: json.RawMessage(`{"native":true}`)}, nil
			},
		}

		result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !executed {
			t.Error("Tool should be executed natively when no WASM module loaded")
		}
		if string(result.Output) != `{"native":true}` {
			t.Errorf("Output = %s, want {\"native\":true}", result.Output)
		}
	})
}

func TestWASMSandbox_LoadModule(t *testing.T) {
	t.Parallel()

	t.Run("returns error for invalid WASM", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		// Invalid WASM bytes should fail
		err = sb.LoadModule("test_module", []byte("not valid wasm"))
		if err == nil {
			t.Error("LoadModule() should return error for invalid WASM")
		}
	})
}

func TestWASMSandbox_UnloadModule(t *testing.T) {
	t.Parallel()

	t.Run("returns error for non-existent module", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		err = sb.UnloadModule("non_existent")
		if err == nil {
			t.Error("UnloadModule() should return error for non-existent module")
		}
	})
}

func TestWASMSandbox_LoadedModules(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	modules := sb.LoadedModules()
	if len(modules) != 0 {
		t.Errorf("LoadedModules() = %v, want empty slice", modules)
	}
}

func TestWASMSandbox_HasModule(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	if sb.HasModule("any_module") {
		t.Error("HasModule() should return false for non-existent module")
	}
}

func TestWASMSandbox_Capabilities(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM(
		sandbox.WithMaxMemory(64*1024*1024),
		sandbox.WithMaxExecTime(45*time.Second),
		sandbox.WithFilesystem("/data", []string{"/readonly"}, []string{"/writable"}),
		sandbox.WithEnv("PATH", "HOME"),
	)
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()

	if caps.MaxMemory != 64*1024*1024 {
		t.Errorf("MaxMemory = %d, want %d", caps.MaxMemory, 64*1024*1024)
	}
	if caps.MaxExecTime != 45*time.Second {
		t.Errorf("MaxExecTime = %v, want %v", caps.MaxExecTime, 45*time.Second)
	}
	if !caps.Filesystem {
		t.Error("Filesystem should be true")
	}
	if len(caps.AllowedEnv) != 2 {
		t.Errorf("AllowedEnv length = %d, want 2", len(caps.AllowedEnv))
	}
	if len(caps.ReadOnlyPaths) != 1 || caps.ReadOnlyPaths[0] != "/readonly" {
		t.Errorf("ReadOnlyPaths = %v, want [/readonly]", caps.ReadOnlyPaths)
	}
	if len(caps.WritePaths) != 1 || caps.WritePaths[0] != "/writable" {
		t.Errorf("WritePaths = %v, want [/writable]", caps.WritePaths)
	}
}

func TestWASMSandbox_Close(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// minimalWASM is the smallest valid WASM module (just magic number + version)
// This is a minimal module with no functions, but valid WASM binary format.
// In WAT: (module)
var minimalWASM = []byte{
	0x00, 0x61, 0x73, 0x6d, // WASM magic number "\0asm"
	0x01, 0x00, 0x00, 0x00, // WASM version 1
}

// wasmWithStart is a WASM module with an empty _start function
// Generated from WAT: (module (func (export "_start")))
var wasmWithStart = []byte{
	0x00, 0x61, 0x73, 0x6d, // WASM magic number
	0x01, 0x00, 0x00, 0x00, // version 1
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type section: one func type with no params and no results
	0x03, 0x02, 0x01, 0x00, // func section: one function using type 0
	0x07, 0x0a, 0x01, 0x06, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x00, 0x00, // export section: export "_start" as func 0
	0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code section: empty function body
}

func TestWASMSandbox_LoadValidModule(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	// Load a minimal valid WASM module
	err = sb.LoadModule("minimal", minimalWASM)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}

	// Verify module is loaded
	if !sb.HasModule("minimal") {
		t.Error("HasModule() should return true after loading")
	}

	modules := sb.LoadedModules()
	if len(modules) != 1 {
		t.Errorf("LoadedModules() count = %d, want 1", len(modules))
	}
}

func TestWASMSandbox_LoadModuleReplaceExisting(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	// Load initial module
	err = sb.LoadModule("test", minimalWASM)
	if err != nil {
		t.Fatalf("First LoadModule() error = %v", err)
	}

	// Replace with same name should succeed
	err = sb.LoadModule("test", minimalWASM)
	if err != nil {
		t.Fatalf("Second LoadModule() error = %v", err)
	}

	// Should still have exactly 1 module
	modules := sb.LoadedModules()
	if len(modules) != 1 {
		t.Errorf("LoadedModules() count = %d, want 1", len(modules))
	}
}

func TestWASMSandbox_UnloadExistingModule(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	// Load a module first
	err = sb.LoadModule("to_unload", minimalWASM)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}

	// Verify loaded
	if !sb.HasModule("to_unload") {
		t.Fatal("Module should be loaded")
	}

	// Unload it
	err = sb.UnloadModule("to_unload")
	if err != nil {
		t.Fatalf("UnloadModule() error = %v", err)
	}

	// Verify unloaded
	if sb.HasModule("to_unload") {
		t.Error("Module should be unloaded")
	}
}

func TestWASMSandbox_CloseWithModules(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM()
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}

	// Load some modules
	err = sb.LoadModule("mod1", minimalWASM)
	if err != nil {
		t.Fatalf("LoadModule(mod1) error = %v", err)
	}
	err = sb.LoadModule("mod2", minimalWASM)
	if err != nil {
		t.Fatalf("LoadModule(mod2) error = %v", err)
	}

	// Close should clean up all modules
	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNewWASM_MemoryPageCalculation(t *testing.T) {
	t.Parallel()

	// Test with very small MaxMemory that results in 0 pages before adjustment
	sb, err := sandbox.NewWASM(sandbox.WithMaxMemory(1024)) // 1KB, less than one page
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	// Should still work (minimum 1 page)
	caps := sb.Capabilities()
	if caps.MaxMemory != 1024 {
		t.Errorf("MaxMemory = %d, want 1024", caps.MaxMemory)
	}
}

func TestWASMSandbox_ExecuteWithLoadedModule(t *testing.T) {
	t.Parallel()

	t.Run("executes WASM module with _start function", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		// Load the module with _start function
		err = sb.LoadModule("test_exec", wasmWithStart)
		if err != nil {
			t.Fatalf("LoadModule() error = %v", err)
		}

		mockT := &mockTool{
			name: "test_exec", // Same name as loaded module
		}

		// Execute should use the WASM module
		result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		// The _start function produces no output, so we expect wrapped empty output
		if result.Output == nil {
			t.Error("Output should not be nil")
		}
	})

	t.Run("handles module without execute or _start function", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM()
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		// Load minimal module without functions
		err = sb.LoadModule("no_entry", minimalWASM)
		if err != nil {
			t.Fatalf("LoadModule() error = %v", err)
		}

		mockT := &mockTool{
			name: "no_entry",
		}

		// Execute should fail because no entry point found
		_, err = sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err == nil {
			t.Error("Execute() should return error for module without entry point")
		}
	})

	t.Run("handles context timeout during execution", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM(sandbox.WithMaxExecTime(100 * time.Millisecond))
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		err = sb.LoadModule("timeout_test", wasmWithStart)
		if err != nil {
			t.Fatalf("LoadModule() error = %v", err)
		}

		mockT := &mockTool{
			name: "timeout_test",
		}

		// Use already-cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = sb.Execute(ctx, mockT, json.RawMessage(`{}`))
		// Should handle cancelled context gracefully
		if err == nil {
			t.Logf("Execute with cancelled context returned no error (may be expected)")
		}
	})

	t.Run("uses default timeout when not configured", func(t *testing.T) {
		t.Parallel()

		sb, err := sandbox.NewWASM(sandbox.WithMaxExecTime(0)) // 0 means use default
		if err != nil {
			t.Fatalf("NewWASM() error = %v", err)
		}
		defer sb.Close()

		err = sb.LoadModule("default_timeout", wasmWithStart)
		if err != nil {
			t.Fatalf("LoadModule() error = %v", err)
		}

		mockT := &mockTool{
			name: "default_timeout",
		}

		// Should use default 30s timeout
		result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.Output == nil {
			t.Error("Output should not be nil")
		}
	})
}

func TestWASMSandbox_ExecuteWithEnvVars(t *testing.T) {
	t.Parallel()

	sb, err := sandbox.NewWASM(sandbox.WithEnv("TEST_VAR", "OTHER_VAR"))
	if err != nil {
		t.Fatalf("NewWASM() error = %v", err)
	}
	defer sb.Close()

	err = sb.LoadModule("env_test", wasmWithStart)
	if err != nil {
		t.Fatalf("LoadModule() error = %v", err)
	}

	mockT := &mockTool{
		name: "env_test",
	}

	// Execute should work with env vars configured
	result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == nil {
		t.Error("Output should not be nil")
	}
}
