// Package wasm provides WebAssembly-based tool sandboxing using wazero.
package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox"
)

// Sandbox provides WASM-based tool execution isolation.
type Sandbox struct {
	runtime wazero.Runtime
	config  sandbox.Config
	modules sync.Map // map[string]api.Module - compiled modules cache
	mu      sync.Mutex
}

// New creates a new WASM sandbox.
func New(opts ...sandbox.Option) (*Sandbox, error) {
	cfg := sandbox.Config{
		MaxMemory:   64 * 1024 * 1024, // 64MB default
		MaxExecTime: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Create runtime with memory limits
	runtimeConfig := wazero.NewRuntimeConfig()
	if cfg.MaxMemory > 0 {
		// WASM memory is in pages of 64KB
		maxPages := uint32(cfg.MaxMemory / 65536)
		if maxPages == 0 {
			maxPages = 1
		}
		runtimeConfig = runtimeConfig.WithMemoryLimitPages(maxPages)
	}

	runtime := wazero.NewRuntimeWithConfig(context.Background(), runtimeConfig)

	// Instantiate WASI for standard I/O
	if _, err := wasi_snapshot_preview1.Instantiate(context.Background(), runtime); err != nil {
		_ = runtime.Close(context.Background())
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	return &Sandbox{
		runtime: runtime,
		config:  cfg,
	}, nil
}

// Execute runs a tool in the WASM sandbox.
func (s *Sandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	// Apply timeout
	if s.config.MaxExecTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.MaxExecTime)
		defer cancel()
	}

	// For tools that provide WASM modules, we can execute them directly
	// Otherwise, we execute the tool normally but with resource constraints
	wasmTool, ok := t.(WASMTool)
	if ok {
		return s.executeWASM(ctx, wasmTool, input)
	}

	// Fall back to direct execution with timeout (basic sandboxing)
	return t.Execute(ctx, input)
}

// WASMTool is a tool that provides a WASM module for execution.
type WASMTool interface {
	tool.Tool
	// WASMModule returns the compiled WASM module bytes.
	WASMModule() []byte
	// EntryPoint returns the function name to call.
	EntryPoint() string
}

// executeWASM executes a WASM module.
func (s *Sandbox) executeWASM(ctx context.Context, t WASMTool, input json.RawMessage) (tool.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	toolName := t.Name()
	wasmBytes := t.WASMModule()
	entryPoint := t.EntryPoint()

	// Check module cache
	var mod api.Module
	if cached, ok := s.modules.Load(toolName); ok {
		mod = cached.(api.Module)
	} else {
		// Compile the module
		compiled, err := s.runtime.CompileModule(ctx, wasmBytes)
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to compile WASM module: %w", err)
		}

		// Configure module
		moduleConfig := wazero.NewModuleConfig().
			WithName(toolName).
			WithStartFunctions() // Don't auto-start

		// Instantiate the module
		mod, err = s.runtime.InstantiateModule(ctx, compiled, moduleConfig)
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to instantiate WASM module: %w", err)
		}

		s.modules.Store(toolName, mod)
	}

	// Get the entry point function
	fn := mod.ExportedFunction(entryPoint)
	if fn == nil {
		return tool.Result{}, fmt.Errorf("entry point function %q not found", entryPoint)
	}

	// Allocate memory for input
	inputLen := uint64(len(input))

	// Get memory allocator if available
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		// Simple execution without input passing
		results, err := fn.Call(ctx)
		if err != nil {
			return tool.Result{}, fmt.Errorf("WASM execution failed: %w", err)
		}

		if len(results) > 0 {
			return tool.Result{Output: json.RawMessage(fmt.Sprintf(`{"result":%d}`, results[0]))}, nil
		}
		return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
	}

	// Allocate memory for input
	allocResults, err := malloc.Call(ctx, inputLen)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to allocate memory: %w", err)
	}
	inputPtr := allocResults[0]

	// Write input to WASM memory
	memory := mod.Memory()
	if memory == nil {
		return tool.Result{}, fmt.Errorf("module has no memory")
	}

	if !memory.Write(uint32(inputPtr), input) {
		return tool.Result{}, fmt.Errorf("failed to write input to memory")
	}

	// Call the function with input pointer and length
	results, err := fn.Call(ctx, inputPtr, inputLen)
	if err != nil {
		return tool.Result{}, fmt.Errorf("WASM execution failed: %w", err)
	}

	// Read result from memory
	if len(results) >= 2 {
		resultPtr := uint32(results[0])
		resultLen := uint32(results[1])

		resultBytes, ok := memory.Read(resultPtr, resultLen)
		if !ok {
			return tool.Result{}, fmt.Errorf("failed to read result from memory")
		}

		return tool.Result{Output: resultBytes}, nil
	}

	return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
}

// Capabilities returns the sandbox capabilities.
func (s *Sandbox) Capabilities() sandbox.Capabilities {
	return sandbox.Capabilities{
		Network:       s.config.AllowNetwork,
		Filesystem:    s.config.AllowFilesystem,
		MaxMemory:     s.config.MaxMemory,
		MaxExecTime:   s.config.MaxExecTime,
		AllowedEnv:    s.config.AllowedEnv,
		ReadOnlyPaths: s.config.ReadOnlyPaths,
		WritePaths:    s.config.WritePaths,
	}
}

// Close releases sandbox resources.
func (s *Sandbox) Close() error {
	return s.runtime.Close(context.Background())
}

// Stats returns current sandbox statistics.
func (s *Sandbox) Stats() Stats {
	var moduleCount int
	s.modules.Range(func(_, _ interface{}) bool {
		moduleCount++
		return true
	})

	return Stats{
		LoadedModules: moduleCount,
		MaxMemory:     s.config.MaxMemory,
		MaxExecTime:   s.config.MaxExecTime,
	}
}

// Stats contains sandbox statistics.
type Stats struct {
	LoadedModules int           `json:"loaded_modules"`
	MaxMemory     int64         `json:"max_memory"`
	MaxExecTime   time.Duration `json:"max_exec_time"`
}
