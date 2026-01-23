// Package filesystem provides filesystem operation tools with enhanced security and features.
package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/fsnotify/fsnotify"
)

// Config configures the filesystem pack.
type Config struct {
	// RootDir restricts operations to this directory and its subdirectories.
	// Empty means no restriction (use with caution).
	RootDir string

	// AllowDelete enables delete operations.
	AllowDelete bool

	// MaxFileSize limits the size of files that can be read (bytes).
	MaxFileSize int64

	// Timeout for operations.
	Timeout time.Duration

	// AllowSymlinks allows following symbolic links.
	AllowSymlinks bool

	// WatchBufferSize is the buffer size for watch events.
	WatchBufferSize int
}

// Option configures the filesystem pack.
type Option func(*Config)

// WithRootDir restricts operations to a directory.
func WithRootDir(dir string) Option {
	return func(c *Config) {
		c.RootDir = dir
	}
}

// WithDeleteAccess enables delete operations.
func WithDeleteAccess() Option {
	return func(c *Config) {
		c.AllowDelete = true
	}
}

// WithMaxFileSize sets the maximum file size for reads.
func WithMaxFileSize(size int64) Option {
	return func(c *Config) {
		c.MaxFileSize = size
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithSymlinks enables following symbolic links.
func WithSymlinks() Option {
	return func(c *Config) {
		c.AllowSymlinks = true
	}
}

// WithWatchBufferSize sets the watch event buffer size.
func WithWatchBufferSize(size int) Option {
	return func(c *Config) {
		c.WatchBufferSize = size
	}
}

// New creates the filesystem pack.
func New(opts ...Option) (*pack.Pack, error) {
	cfg := Config{
		AllowDelete:     false,
		MaxFileSize:     10 * 1024 * 1024, // 10MB default
		Timeout:         60 * time.Second,
		AllowSymlinks:   false,
		WatchBufferSize: 100,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Validate and normalize root directory
	if cfg.RootDir != "" {
		absRoot, err := filepath.Abs(cfg.RootDir)
		if err != nil {
			return nil, fmt.Errorf("invalid root directory: %w", err)
		}
		cfg.RootDir = absRoot

		// Verify root exists
		info, err := os.Stat(cfg.RootDir)
		if err != nil {
			return nil, fmt.Errorf("root directory does not exist: %w", err)
		}
		if !info.IsDir() {
			return nil, errors.New("root path is not a directory")
		}
	}

	builder := pack.NewBuilder("filesystem").
		WithDescription("Filesystem operations with security controls").
		WithVersion("1.0.0").
		AddTools(
			readTool(&cfg),
			writeTool(&cfg),
			listTool(&cfg),
			watchTool(&cfg),
		).
		AllowInState(agent.StateExplore, "fs_read", "fs_list").
		AllowInState(agent.StateValidate, "fs_read", "fs_list")

	// Add delete tool if enabled
	if cfg.AllowDelete {
		builder = builder.AddTools(deleteTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "fs_read", "fs_write", "fs_list", "fs_delete", "fs_watch")
	} else {
		builder = builder.AllowInState(agent.StateAct, "fs_read", "fs_write", "fs_list", "fs_watch")
	}

	return builder.Build(), nil
}

// validatePath ensures a path is within the allowed root directory.
func validatePath(cfg *Config, path string) (string, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// If no root restriction, allow any path
	if cfg.RootDir == "" {
		return absPath, nil
	}

	// Check if path is within root
	rel, err := filepath.Rel(cfg.RootDir, absPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Check for path escape attempts
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("path escapes root directory")
	}

	// Check for symlinks if not allowed
	if !cfg.AllowSymlinks {
		// Walk up the path checking for symlinks
		parts := strings.Split(rel, string(filepath.Separator))
		current := cfg.RootDir
		for _, part := range parts {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if err != nil {
				if os.IsNotExist(err) {
					break // Path doesn't exist yet, OK
				}
				return "", err
			}
			if info.Mode()&fs.ModeSymlink != 0 {
				return "", errors.New("symbolic links not allowed")
			}
		}
	}

	return absPath, nil
}

// --- fs_read tool ---

type readInput struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Length int64  `json:"length,omitempty"`
}

type readOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
	Mode      string `json:"mode"`
	ModTime   string `json:"mod_time"`
}

func readTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("fs_read").
		WithDescription("Read contents of a file with size limits").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in readInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Apply timeout
			_, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Validate path
			absPath, err := validatePath(cfg, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Get file info
			info, err := os.Stat(absPath)
			if err != nil {
				return tool.Result{}, err
			}

			if info.IsDir() {
				return tool.Result{}, errors.New("cannot read directory")
			}

			// Open file
			f, err := os.Open(absPath) // #nosec G304 -- path validated above
			if err != nil {
				return tool.Result{}, err
			}
			defer f.Close()

			// Handle offset
			if in.Offset > 0 {
				if _, err := f.Seek(in.Offset, 0); err != nil {
					return tool.Result{}, err
				}
			}

			// Determine read length
			readLen := cfg.MaxFileSize
			if in.Length > 0 && in.Length < readLen {
				readLen = in.Length
			}

			// Read content
			buf := make([]byte, readLen+1) // +1 to detect truncation
			n, err := f.Read(buf)
			if err != nil && err.Error() != "EOF" {
				return tool.Result{}, err
			}

			truncated := n > int(readLen)
			if truncated {
				n = int(readLen)
			}

			out := readOutput{
				Path:      absPath,
				Content:   string(buf[:n]),
				Size:      int64(n),
				Truncated: truncated,
				Mode:      info.Mode().String(),
				ModTime:   info.ModTime().Format(time.RFC3339),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- fs_write tool ---

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
	Mode    int    `json:"mode,omitempty"`
}

type writeOutput struct {
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Created bool   `json:"created"`
}

func writeTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("fs_write").
		WithDescription("Write content to a file").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in writeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Apply timeout
			_, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Validate path
			absPath, err := validatePath(cfg, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Check if file exists
			_, statErr := os.Stat(absPath)
			created := os.IsNotExist(statErr)

			// Ensure directory exists
			dir := filepath.Dir(absPath)
			if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301 -- restrictive permissions
				return tool.Result{}, err
			}

			// Determine file mode
			mode := fs.FileMode(0600) // Default restrictive
			if in.Mode != 0 {
				mode = fs.FileMode(in.Mode)
			}

			// Write file
			flags := os.O_WRONLY | os.O_CREATE
			if in.Append {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}

			f, err := os.OpenFile(absPath, flags, mode) // #nosec G304 -- path validated above
			if err != nil {
				return tool.Result{}, err
			}
			defer f.Close()

			n, err := f.WriteString(in.Content)
			if err != nil {
				return tool.Result{}, err
			}

			out := writeOutput{
				Path:    absPath,
				Bytes:   n,
				Created: created,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- fs_list tool ---

type listInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
	MaxDepth  int    `json:"max_depth,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
}

type fileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

type listOutput struct {
	Path       string      `json:"path"`
	Entries    []fileEntry `json:"entries"`
	Count      int         `json:"count"`
	Truncated  bool        `json:"truncated,omitempty"`
	TotalBytes int64       `json:"total_bytes"`
}

func listTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("fs_list").
		WithDescription("List directory contents with optional recursion").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in listInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Validate path
			absPath, err := validatePath(cfg, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Verify it's a directory
			info, err := os.Stat(absPath)
			if err != nil {
				return tool.Result{}, err
			}
			if !info.IsDir() {
				return tool.Result{}, errors.New("path is not a directory")
			}

			maxDepth := in.MaxDepth
			if maxDepth == 0 {
				maxDepth = 10 // Default max depth
			}

			var entries []fileEntry
			var totalBytes int64
			const maxEntries = 10000

			if in.Recursive {
				err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil // Skip errors, continue walking
					}

					// Check context
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}

					// Calculate depth
					rel, _ := filepath.Rel(absPath, path)
					depth := strings.Count(rel, string(filepath.Separator))
					if depth > maxDepth {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					// Skip root
					if path == absPath {
						return nil
					}

					// Apply pattern filter
					if in.Pattern != "" {
						matched, _ := filepath.Match(in.Pattern, d.Name())
						if !matched {
							return nil
						}
					}

					// Check limit
					if len(entries) >= maxEntries {
						return filepath.SkipAll
					}

					info, _ := d.Info()
					var size int64
					var modTime time.Time
					var mode fs.FileMode
					if info != nil {
						size = info.Size()
						modTime = info.ModTime()
						mode = info.Mode()
					}

					totalBytes += size
					entries = append(entries, fileEntry{
						Name:    d.Name(),
						Path:    path,
						IsDir:   d.IsDir(),
						Size:    size,
						Mode:    mode.String(),
						ModTime: modTime.Format(time.RFC3339),
					})

					return nil
				})
			} else {
				dirEntries, err := os.ReadDir(absPath)
				if err != nil {
					return tool.Result{}, err
				}

				for _, d := range dirEntries {
					// Check context
					select {
					case <-ctx.Done():
						return tool.Result{}, ctx.Err()
					default:
					}

					// Apply pattern filter
					if in.Pattern != "" {
						matched, _ := filepath.Match(in.Pattern, d.Name())
						if !matched {
							continue
						}
					}

					// Check limit
					if len(entries) >= maxEntries {
						break
					}

					info, _ := d.Info()
					var size int64
					var modTime time.Time
					var mode fs.FileMode
					if info != nil {
						size = info.Size()
						modTime = info.ModTime()
						mode = info.Mode()
					}

					totalBytes += size
					entries = append(entries, fileEntry{
						Name:    d.Name(),
						Path:    filepath.Join(absPath, d.Name()),
						IsDir:   d.IsDir(),
						Size:    size,
						Mode:    mode.String(),
						ModTime: modTime.Format(time.RFC3339),
					})
				}
			}

			if err != nil && err != filepath.SkipAll {
				return tool.Result{}, err
			}

			out := listOutput{
				Path:       absPath,
				Entries:    entries,
				Count:      len(entries),
				Truncated:  len(entries) >= maxEntries,
				TotalBytes: totalBytes,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- fs_delete tool ---

type deleteInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type deleteOutput struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
	WasDir  bool   `json:"was_dir,omitempty"`
}

func deleteTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("fs_delete").
		WithDescription("Delete a file or directory").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Apply timeout
			_, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			// Validate path
			absPath, err := validatePath(cfg, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Check if exists
			info, err := os.Stat(absPath)
			if os.IsNotExist(err) {
				out := deleteOutput{Path: absPath, Deleted: false}
				data, _ := json.Marshal(out)
				return tool.Result{Output: data}, nil
			}
			if err != nil {
				return tool.Result{}, err
			}

			wasDir := info.IsDir()

			// Delete
			if in.Recursive {
				err = os.RemoveAll(absPath)
			} else {
				err = os.Remove(absPath)
			}

			if err != nil {
				return tool.Result{}, err
			}

			out := deleteOutput{
				Path:    absPath,
				Deleted: true,
				WasDir:  wasDir,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// --- fs_watch tool ---

type watchInput struct {
	Path     string `json:"path"`
	Duration int    `json:"duration_seconds,omitempty"`
}

type watchEvent struct {
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Timestamp string `json:"timestamp"`
}

type watchOutput struct {
	Path   string       `json:"path"`
	Events []watchEvent `json:"events"`
	Count  int          `json:"count"`
}

// activeWatchers tracks active watchers for cleanup
var (
	activeWatchers   = make(map[string]*fsnotify.Watcher)
	activeWatchersMu sync.Mutex
)

func watchTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("fs_watch").
		WithDescription("Watch a directory for changes").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in watchInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Default duration
			duration := time.Duration(in.Duration) * time.Second
			if duration == 0 {
				duration = 10 * time.Second
			}
			if duration > 60*time.Second {
				duration = 60 * time.Second // Cap at 60 seconds
			}

			// Validate path
			absPath, err := validatePath(cfg, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Verify path exists
			info, err := os.Stat(absPath)
			if err != nil {
				return tool.Result{}, err
			}

			// Create watcher
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create watcher: %w", err)
			}

			// Track watcher for cleanup
			watchID := fmt.Sprintf("%s-%d", absPath, time.Now().UnixNano())
			activeWatchersMu.Lock()
			activeWatchers[watchID] = watcher
			activeWatchersMu.Unlock()

			defer func() {
				_ = watcher.Close()
				activeWatchersMu.Lock()
				delete(activeWatchers, watchID)
				activeWatchersMu.Unlock()
			}()

			// Add path to watcher
			if info.IsDir() {
				err = watcher.Add(absPath)
			} else {
				err = watcher.Add(filepath.Dir(absPath))
			}
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to watch path: %w", err)
			}

			// Collect events
			var events []watchEvent
			timeout := time.After(duration)

		watchLoop:
			for {
				select {
				case <-ctx.Done():
					break watchLoop
				case <-timeout:
					break watchLoop
				case event, ok := <-watcher.Events:
					if !ok {
						break watchLoop
					}
					// Filter for specific file if not watching directory
					if !info.IsDir() && event.Name != absPath {
						continue
					}
					events = append(events, watchEvent{
						Name:      event.Name,
						Operation: event.Op.String(),
						Timestamp: time.Now().Format(time.RFC3339),
					})
					if len(events) >= cfg.WatchBufferSize {
						break watchLoop
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						break watchLoop
					}
					// Log error but continue watching
					events = append(events, watchEvent{
						Name:      "error",
						Operation: err.Error(),
						Timestamp: time.Now().Format(time.RFC3339),
					})
				}
			}

			out := watchOutput{
				Path:   absPath,
				Events: events,
				Count:  len(events),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
