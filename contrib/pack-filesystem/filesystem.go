// Package filesystem provides sandboxed file system tools for agent-go.
//
// This pack includes tools for file system operations:
//   - fs_read_file: Read contents of a file
//   - fs_write_file: Write contents to a file
//   - fs_list_dir: List directory contents
//   - fs_stat: Get file or directory metadata
//   - fs_mkdir: Create directories
//   - fs_remove: Remove files or directories
//   - fs_copy: Copy files or directories
//   - fs_move: Move or rename files or directories
//
// All paths are resolved relative to a configured base directory and
// rejected if they escape that sandbox.
package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/sandbox"
)

// Pack returns the filesystem tools pack.
// The baseDir parameter restricts all file operations to the given directory
// to prevent path traversal attacks. All paths provided by tool inputs are
// resolved relative to baseDir.
func Pack(baseDir string) *pack.Pack {
	return pack.NewBuilder("filesystem").
		WithDescription("File system tools for reading, writing, and managing files").
		WithVersion("0.1.0").
		AddTools(
			readFile(baseDir),
			writeFile(baseDir),
			listDir(baseDir),
			stat(baseDir),
			mkdir(baseDir),
			remove(baseDir),
			copyFile(baseDir),
			moveFile(baseDir),
		).
		AllowInState(agent.StateExplore, "fs_read_file", "fs_list_dir", "fs_stat").
		AllowInState(agent.StateAct, "fs_read_file", "fs_write_file", "fs_list_dir", "fs_stat", "fs_mkdir", "fs_remove", "fs_copy", "fs_move").
		AllowInState(agent.StateValidate, "fs_read_file", "fs_list_dir", "fs_stat").
		Build()
}

// --- Path security ---

func safePath(baseDir, userPath string) (string, error) {
	return sandbox.SafePath(baseDir, userPath)
}

// --- Input/Output types ---

type readFileInput struct {
	Path string `json:"path"`
}

type readFileOutput struct {
	Content string `json:"content"`
	Size    int    `json:"size"`
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type writeFileOutput struct {
	BytesWritten int  `json:"bytes_written"`
	Created      bool `json:"created"`
}

type listDirInput struct {
	Path string `json:"path"`
}

type dirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type listDirOutput struct {
	Entries []dirEntry `json:"entries"`
	Count   int        `json:"count"`
}

type statInput struct {
	Path string `json:"path"`
}

type statOutput struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

type mkdirInput struct {
	Path    string `json:"path"`
	Parents bool   `json:"parents,omitempty"`
}

type mkdirOutput struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

type removeInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type removeOutput struct {
	Path    string `json:"path"`
	Removed bool   `json:"removed"`
}

type copyInput struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

type copyOutput struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
	IsDir  bool   `json:"is_dir"`
}

type moveInput struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

type moveOutput struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// --- Tool constructors ---

func readFile(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_read_file").
		WithDescription("Read the contents of a file").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Path to the file to read (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in readFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// #nosec G304 -- path is sanitized via safePath
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			out := readFileOutput{
				Content: string(data),
				Size:    len(data),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func writeFile(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_write_file").
		WithDescription("Write contents to a file, creating it if necessary").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":    json.RawMessage(`{"type":"string","description":"Path to the file to write (relative to base directory)"}`),
			"content": json.RawMessage(`{"type":"string","description":"Content to write"}`),
		}, []string{"path", "content"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in writeFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			_, statErr := os.Stat(fullPath)
			created := os.IsNotExist(statErr)

			if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
			}

			if err := os.WriteFile(fullPath, []byte(in.Content), 0600); err != nil {
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			out := writeFileOutput{
				BytesWritten: len(in.Content),
				Created:      created,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func listDir(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_list_dir").
		WithDescription("List the contents of a directory").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Directory path to list (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in listDirInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list directory: %w", err)
			}

			outEntries := make([]dirEntry, 0, len(entries))
			for _, e := range entries {
				var size int64
				if info, infoErr := e.Info(); infoErr == nil {
					size = info.Size()
				}
				outEntries = append(outEntries, dirEntry{
					Name:  e.Name(),
					IsDir: e.IsDir(),
					Size:  size,
				})
			}

			out := listDirOutput{
				Entries: outEntries,
				Count:   len(outEntries),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func stat(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_stat").
		WithDescription("Get metadata about a file or directory").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Path to stat (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in statInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat path: %w", err)
			}

			out := statOutput{
				Name:    info.Name(),
				Size:    info.Size(),
				Mode:    info.Mode().String(),
				ModTime: info.ModTime().UTC().Format(time.RFC3339),
				IsDir:   info.IsDir(),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func mkdir(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_mkdir").
		WithDescription("Create a directory, optionally with parent directories").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":    json.RawMessage(`{"type":"string","description":"Directory path to create (relative to base directory)"}`),
			"parents": json.RawMessage(`{"type":"boolean","description":"Create parent directories as needed (default: false)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in mkdirInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			_, statErr := os.Stat(fullPath)
			created := os.IsNotExist(statErr)

			if in.Parents {
				if err := os.MkdirAll(fullPath, 0750); err != nil {
					return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
				}
			} else {
				if err := os.Mkdir(fullPath, 0750); err != nil {
					if !os.IsExist(err) {
						return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
					}
					created = false
				}
			}

			out := mkdirOutput{
				Path:    in.Path,
				Created: created,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func remove(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_remove").
		WithDescription("Remove a file or directory").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":      json.RawMessage(`{"type":"string","description":"Path to remove (relative to base directory)"}`),
			"recursive": json.RawMessage(`{"type":"boolean","description":"Remove directories and contents recursively (default: false)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in removeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to remove: %w", err)
			}

			if info.IsDir() && in.Recursive {
				if err := os.RemoveAll(fullPath); err != nil {
					return tool.Result{}, fmt.Errorf("failed to remove: %w", err)
				}
			} else {
				if err := os.Remove(fullPath); err != nil {
					return tool.Result{}, fmt.Errorf("failed to remove: %w", err)
				}
			}

			out := removeOutput{
				Path:    in.Path,
				Removed: true,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func copyFile(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_copy").
		WithDescription("Copy a file or directory to a new location").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"source": json.RawMessage(`{"type":"string","description":"Source path (relative to base directory)"}`),
			"dest":   json.RawMessage(`{"type":"string","description":"Destination path (relative to base directory)"}`),
		}, []string{"source", "dest"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in copyInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			srcPath, err := safePath(baseDir, in.Source)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.Dest)
			if err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(srcPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat source: %w", err)
			}

			if info.IsDir() {
				if err := copyDir(srcPath, dstPath); err != nil {
					return tool.Result{}, err
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
					return tool.Result{}, fmt.Errorf("failed to create destination directory: %w", err)
				}
				if err := copyFileContents(srcPath, dstPath); err != nil {
					return tool.Result{}, err
				}
			}

			out := copyOutput{
				Source: in.Source,
				Dest:   in.Dest,
				IsDir:  info.IsDir(),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func moveFile(baseDir string) tool.Tool {
	return tool.NewBuilder("fs_move").
		WithDescription("Move or rename a file or directory").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"source": json.RawMessage(`{"type":"string","description":"Source path (relative to base directory)"}`),
			"dest":   json.RawMessage(`{"type":"string","description":"Destination path (relative to base directory)"}`),
		}, []string{"source", "dest"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in moveInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			srcPath, err := safePath(baseDir, in.Source)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.Dest)
			if err != nil {
				return tool.Result{}, err
			}

			if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create destination directory: %w", err)
			}

			if err := os.Rename(srcPath, dstPath); err != nil {
				return tool.Result{}, fmt.Errorf("failed to move: %w", err)
			}

			out := moveOutput{
				Source: in.Source,
				Dest:   in.Dest,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// --- Helpers ---

func copyFileContents(src, dst string) error {
	// #nosec G304 -- path is sanitized via safePath by callers
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFileContents(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}
