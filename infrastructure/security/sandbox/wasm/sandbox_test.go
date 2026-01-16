package wasm_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox/wasm"
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

// mockWASMTool implements both tool.Tool and wasm.WASMTool interfaces.
type mockWASMTool struct {
	mockTool
	wasmModule []byte
	entryPoint string
}

func (m *mockWASMTool) WASMModule() []byte { return m.wasmModule }
func (m *mockWASMTool) EntryPoint() string { return m.entryPoint }

// minimalWASMModule returns a minimal valid WASM module that exports a simple function.
// This is a hand-crafted WASM binary that:
// - Has the WASM magic number and version
// - Defines a function type () -> i32
// - Declares one function of that type
// - Exports the function as "run"
// - The function body just returns 42
func minimalWASMModule() []byte {
	return []byte{
		// WASM header
		0x00, 0x61, 0x73, 0x6d, // magic number (\0asm)
		0x01, 0x00, 0x00, 0x00, // version 1

		// Type section (section id = 1)
		0x01,       // section id
		0x05,       // section size (5 bytes)
		0x01,       // number of types
		0x60,       // func type
		0x00,       // 0 params
		0x01, 0x7f, // 1 result (i32)

		// Function section (section id = 3)
		0x03,       // section id
		0x02,       // section size (2 bytes)
		0x01,       // number of functions
		0x00,       // function 0 uses type 0

		// Export section (section id = 7)
		0x07,                                     // section id
		0x07,                                     // section size (7 bytes)
		0x01,                                     // number of exports
		0x03, 0x72, 0x75, 0x6e,                   // export name "run"
		0x00,                                     // export kind (function)
		0x00,                                     // export index (function 0)

		// Code section (section id = 10)
		0x0a,             // section id
		0x06,             // section size (6 bytes)
		0x01,             // number of function bodies
		0x04,             // body size (4 bytes)
		0x00,             // local declaration count
		0x41, 0x2a,       // i32.const 42
		0x0b,             // end
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates sandbox with defaults", func(t *testing.T) {
		t.Parallel()

		sb, err := wasm.New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer sb.Close()

		caps := sb.Capabilities()
		if caps.MaxMemory != 64*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", caps.MaxMemory, 64*1024*1024)
		}
		if caps.MaxExecTime != 30*time.Second {
			t.Errorf("MaxExecTime = %v, want %v", caps.MaxExecTime, 30*time.Second)
		}
	})

	t.Run("creates sandbox with options", func(t *testing.T) {
		t.Parallel()

		sb, err := wasm.New(
			sandbox.WithMaxMemory(128*1024*1024),
			sandbox.WithMaxExecTime(60*time.Second),
			sandbox.WithNetwork(),
			sandbox.WithFilesystem("/data", []string{"/etc"}, []string{"/tmp"}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer sb.Close()

		caps := sb.Capabilities()
		if caps.MaxMemory != 128*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", caps.MaxMemory, 128*1024*1024)
		}
		if caps.MaxExecTime != 60*time.Second {
			t.Errorf("MaxExecTime = %v, want %v", caps.MaxExecTime, 60*time.Second)
		}
		if !caps.Network {
			t.Error("Network should be true")
		}
		if !caps.Filesystem {
			t.Error("Filesystem should be true")
		}
	})
}

func TestSandbox_Execute(t *testing.T) {
	t.Parallel()

	t.Run("executes non-wasm tool with timeout", func(t *testing.T) {
		t.Parallel()

		sb, err := wasm.New(sandbox.WithMaxExecTime(5 * time.Second))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer sb.Close()

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

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		sb, err := wasm.New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer sb.Close()

		mockT := &mockTool{
			name: "slow",
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				select {
				case <-ctx.Done():
					return tool.Result{}, ctx.Err()
				case <-time.After(10 * time.Second):
					return tool.Result{Output: json.RawMessage(`{"done":true}`)}, nil
				}
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = sb.Execute(ctx, mockT, json.RawMessage(`{}`))
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

func TestSandbox_Stats(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New(
		sandbox.WithMaxMemory(32*1024*1024),
		sandbox.WithMaxExecTime(15*time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	stats := sb.Stats()

	if stats.LoadedModules != 0 {
		t.Errorf("LoadedModules = %d, want 0", stats.LoadedModules)
	}
	if stats.MaxMemory != 32*1024*1024 {
		t.Errorf("MaxMemory = %d, want %d", stats.MaxMemory, 32*1024*1024)
	}
	if stats.MaxExecTime != 15*time.Second {
		t.Errorf("MaxExecTime = %v, want %v", stats.MaxExecTime, 15*time.Second)
	}
}

func TestSandbox_Close(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNew_WithSmallMemory(t *testing.T) {
	t.Parallel()

	// Test with very small memory (less than one WASM page)
	sb, err := wasm.New(sandbox.WithMaxMemory(1024))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()
	if caps.MaxMemory != 1024 {
		t.Errorf("MaxMemory = %d, want 1024", caps.MaxMemory)
	}
}

func TestNew_WithEnvOptions(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New(
		sandbox.WithEnv("PATH", "HOME", "USER"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()
	if len(caps.AllowedEnv) != 3 {
		t.Errorf("AllowedEnv length = %d, want 3", len(caps.AllowedEnv))
	}
}

func TestSandbox_Execute_WithError(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New(sandbox.WithMaxExecTime(5 * time.Second))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	expectedErr := errors.New("tool failed")
	mockT := &mockTool{
		name: "failing",
		execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, expectedErr
		},
	}

	_, err = sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err == nil {
		t.Error("Execute() expected error")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Execute() error = %v, want %v", err, expectedErr)
	}
}

func TestSandbox_Execute_WithWASMTool(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New(sandbox.WithMaxExecTime(10 * time.Second))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	wasmTool := &mockWASMTool{
		mockTool: mockTool{
			name: "wasm-test",
		},
		wasmModule: minimalWASMModule(),
		entryPoint: "run",
	}

	result, err := sb.Execute(context.Background(), wasmTool, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// The function returns 42, so we expect {"result":42}
	expected := `{"result":42}`
	if string(result.Output) != expected {
		t.Errorf("Output = %s, want %s", result.Output, expected)
	}
}

func TestSandbox_Execute_WithWASMTool_InvalidModule(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	wasmTool := &mockWASMTool{
		mockTool: mockTool{
			name: "invalid-wasm",
		},
		wasmModule: []byte{0x00, 0x01, 0x02, 0x03}, // Invalid WASM
		entryPoint: "run",
	}

	_, err = sb.Execute(context.Background(), wasmTool, json.RawMessage(`{}`))
	if err == nil {
		t.Error("Execute() expected error for invalid WASM module")
	}
}

func TestSandbox_Execute_WithWASMTool_MissingEntryPoint(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	wasmTool := &mockWASMTool{
		mockTool: mockTool{
			name: "missing-entry",
		},
		wasmModule: minimalWASMModule(),
		entryPoint: "nonexistent", // Function doesn't exist
	}

	_, err = sb.Execute(context.Background(), wasmTool, json.RawMessage(`{}`))
	if err == nil {
		t.Error("Execute() expected error for missing entry point")
	}
}

func TestSandbox_Execute_WithWASMTool_ModuleCaching(t *testing.T) {
	t.Parallel()

	sb, err := wasm.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	wasmTool := &mockWASMTool{
		mockTool: mockTool{
			name: "cached-wasm",
		},
		wasmModule: minimalWASMModule(),
		entryPoint: "run",
	}

	// Execute twice - second should use cached module
	_, err = sb.Execute(context.Background(), wasmTool, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("First Execute() error = %v", err)
	}

	// Check stats show loaded module
	stats := sb.Stats()
	if stats.LoadedModules != 1 {
		t.Errorf("LoadedModules = %d, want 1", stats.LoadedModules)
	}

	// Execute again - should use cached module
	_, err = sb.Execute(context.Background(), wasmTool, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Second Execute() error = %v", err)
	}

	// Stats should still show 1 loaded module (cached)
	stats = sb.Stats()
	if stats.LoadedModules != 1 {
		t.Errorf("LoadedModules after cache hit = %d, want 1", stats.LoadedModules)
	}
}

func TestSandbox_Execute_NoTimeout(t *testing.T) {
	t.Parallel()

	// Create sandbox with zero timeout (no timeout enforcement)
	sb, err := wasm.New(sandbox.WithMaxExecTime(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	executed := false
	mockT := &mockTool{
		name: "no-timeout",
		execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			executed = true
			return tool.Result{Output: json.RawMessage(`{"ok":true}`)}, nil
		},
	}

	_, err = sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !executed {
		t.Error("Tool was not executed")
	}
}

func TestSandbox_Capabilities_ReadOnlyPaths(t *testing.T) {
	t.Parallel()

	readOnlyPaths := []string{"/etc", "/usr/share"}
	writePaths := []string{"/tmp", "/var/log"}

	sb, err := wasm.New(
		sandbox.WithFilesystem("/sandbox", readOnlyPaths, writePaths),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()

	if len(caps.ReadOnlyPaths) != 2 {
		t.Errorf("ReadOnlyPaths length = %d, want 2", len(caps.ReadOnlyPaths))
	}
	if len(caps.WritePaths) != 2 {
		t.Errorf("WritePaths length = %d, want 2", len(caps.WritePaths))
	}

	// Verify specific paths
	foundEtc := false
	for _, p := range caps.ReadOnlyPaths {
		if p == "/etc" {
			foundEtc = true
			break
		}
	}
	if !foundEtc {
		t.Error("ReadOnlyPaths should contain /etc")
	}
}
