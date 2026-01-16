package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

var (
	// ErrWASMTimeout indicates the WASM execution exceeded time limit.
	ErrWASMTimeout = errors.New("WASM execution timeout")

	// ErrWASMMemoryLimit indicates the WASM module exceeded memory limit.
	ErrWASMMemoryLimit = errors.New("WASM memory limit exceeded")

	// ErrInvalidWASM indicates the provided WASM module is invalid.
	ErrInvalidWASM = errors.New("invalid WASM module")

	// ErrModuleNotFound indicates the requested WASM module was not found.
	ErrModuleNotFound = errors.New("WASM module not found")
)

// WASMSandbox provides isolated execution using WebAssembly.
// It implements the Sandbox interface for running tools in WASM.
type WASMSandbox struct {
	runtime wazero.Runtime
	config  Config
	mu      sync.RWMutex
	modules map[string]wazero.CompiledModule
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
}

// WASMConfig extends Config with WASM-specific options.
type WASMConfig struct {
	Config

	// MaxMemoryPages sets the maximum memory pages (64KB each).
	// Default: 256 (16MB)
	MaxMemoryPages uint32
}

// DefaultWASMConfig returns a secure default WASM configuration.
func DefaultWASMConfig() WASMConfig {
	return WASMConfig{
		Config: Config{
			MaxMemory:   16 * 1024 * 1024, // 16MB
			MaxExecTime: 30 * time.Second,
		},
		MaxMemoryPages: 256, // 16MB (256 * 64KB)
	}
}

// NewWASM creates a new WASM sandbox with the given configuration.
func NewWASM(opts ...Option) (*WASMSandbox, error) {
	cfg := DefaultWASMConfig()
	for _, opt := range opts {
		opt(&cfg.Config)
	}

	// Calculate memory pages from MaxMemory if set
	if cfg.MaxMemory > 0 && cfg.MaxMemoryPages == 0 {
		pages := cfg.MaxMemory / (64 * 1024)
		// Clamp to uint32 max to prevent overflow
		if pages > math.MaxUint32 {
			pages = math.MaxUint32
		}
		cfg.MaxMemoryPages = uint32(pages) // #nosec G115 -- bounds checked above
		if cfg.MaxMemoryPages == 0 {
			cfg.MaxMemoryPages = 1
		}
	}

	// Create wazero runtime with memory limits
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(cfg.MaxMemoryPages)

	ctx := context.Background()
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// Instantiate WASI for basic system calls
	_, err := wasi_snapshot_preview1.Instantiate(ctx, runtime)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	return &WASMSandbox{
		runtime: runtime,
		config:  cfg.Config,
		modules: make(map[string]wazero.CompiledModule),
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
	}, nil
}

// LoadModule loads a WASM module into the sandbox for later execution.
func (s *WASMSandbox) LoadModule(name string, wasmBytes []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	compiled, err := s.runtime.CompileModule(context.Background(), wasmBytes)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidWASM, err)
	}

	// Close existing module if any
	if existing, exists := s.modules[name]; exists {
		_ = existing.Close(context.Background())
	}

	s.modules[name] = compiled
	return nil
}

// UnloadModule removes a WASM module from the sandbox.
func (s *WASMSandbox) UnloadModule(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	module, exists := s.modules[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrModuleNotFound, name)
	}

	if err := module.Close(context.Background()); err != nil {
		return fmt.Errorf("failed to close module: %w", err)
	}
	delete(s.modules, name)
	return nil
}

// Execute runs a tool in the WASM sandbox.
// Note: This requires the tool to have a corresponding WASM module loaded.
// The tool's name is used to look up the WASM module.
func (s *WASMSandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	s.mu.RLock()
	compiled, exists := s.modules[t.Name()]
	s.mu.RUnlock()

	if !exists {
		// If no WASM module is loaded for this tool, execute natively
		// This allows gradual migration to sandboxed execution
		return t.Execute(ctx, input)
	}

	// Create execution context with timeout
	execTimeout := s.config.MaxExecTime
	if execTimeout == 0 {
		execTimeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// Reset output buffers
	s.stdout.Reset()
	s.stderr.Reset()

	// Configure module instance
	moduleConfig := wazero.NewModuleConfig().
		WithStdout(s.stdout).
		WithStderr(s.stderr).
		WithStartFunctions() // Don't run _start automatically

	// Add allowed environment variables
	for _, env := range s.config.AllowedEnv {
		// Format: VAR=value or just VAR (inherit from environment)
		moduleConfig = moduleConfig.WithEnv(env, "")
	}

	// Instantiate the module
	mod, err := s.runtime.InstantiateModule(execCtx, compiled, moduleConfig)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return tool.Result{}, ErrWASMTimeout
		}
		return tool.Result{}, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer func() { _ = mod.Close(execCtx) }()

	// Look for a standard entry point
	// Try "execute" first, then "_start"
	fn := mod.ExportedFunction("execute")
	if fn == nil {
		fn = mod.ExportedFunction("_start")
	}
	if fn == nil {
		return tool.Result{}, fmt.Errorf("no execute or _start function found in module %s", t.Name())
	}

	// For modules with memory allocation support
	malloc := mod.ExportedFunction("malloc")
	free := mod.ExportedFunction("free")

	var result []byte

	if malloc != nil && free != nil {
		// Allocate input in WASM memory
		inputLen := uint64(len(input))
		inputPtrResult, err := malloc.Call(execCtx, inputLen)
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to allocate input: %w", err)
		}
		// Validate pointer fits in uint32 (WASM memory addresses are 32-bit)
		if inputPtrResult[0] > math.MaxUint32 {
			return tool.Result{}, fmt.Errorf("memory pointer exceeds 32-bit address space")
		}
		inputPtr := uint32(inputPtrResult[0]) // #nosec G115 -- bounds checked above
		defer func() { _, _ = free.Call(execCtx, uint64(inputPtr)) }()

		// Write input to WASM memory
		if !mod.Memory().Write(inputPtr, input) {
			return tool.Result{}, ErrWASMMemoryLimit
		}

		// Call with input pointer and length
		_, err = fn.Call(execCtx, uint64(inputPtr), inputLen)
		if err != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				return tool.Result{}, ErrWASMTimeout
			}
			return tool.Result{}, fmt.Errorf("WASM execution failed: %w", err)
		}

		// Get result from stdout
		result = s.stdout.Bytes()
	} else {
		// Simple execution without memory management
		// Input passed via stdin would need different setup
		_, err = fn.Call(execCtx)
		if err != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				return tool.Result{}, ErrWASMTimeout
			}
			return tool.Result{}, fmt.Errorf("WASM execution failed: %w", err)
		}
		result = s.stdout.Bytes()
	}

	// Parse result as JSON if possible, otherwise return raw output
	var output json.RawMessage
	if json.Valid(result) {
		output = result
	} else {
		// Wrap non-JSON output in a simple structure
		wrapped := map[string]string{"output": string(result)}
		output, _ = json.Marshal(wrapped)
	}

	return tool.Result{Output: output}, nil
}

// Capabilities returns the sandbox's capabilities.
func (s *WASMSandbox) Capabilities() Capabilities {
	return Capabilities{
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
func (s *WASMSandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	// Close all loaded modules
	for name, mod := range s.modules {
		if err := mod.Close(ctx); err != nil {
			// Log but continue closing other modules
			fmt.Printf("warning: failed to close module %s: %v\n", name, err)
		}
		delete(s.modules, name)
	}

	// Close the runtime
	return s.runtime.Close(ctx)
}

// LoadedModules returns the names of all loaded WASM modules.
func (s *WASMSandbox) LoadedModules() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.modules))
	for name := range s.modules {
		names = append(names, name)
	}
	return names
}

// HasModule checks if a module with the given name is loaded.
func (s *WASMSandbox) HasModule(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.modules[name]
	return exists
}
