// Package fileops provides file operation tools.
package fileops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// New creates the fileops pack.
func New() *pack.Pack {
	return pack.NewBuilder("fileops").
		WithDescription("File system operations").
		WithVersion("1.0.0").
		AddTools(
			readFileTool(),
			writeFileTool(),
			listDirTool(),
			fileExistsTool(),
			mkdirTool(),
			deleteTool(),
		).
		AllowInState(agent.StateExplore, "read_file", "list_dir", "file_exists").
		AllowInState(agent.StateAct, "read_file", "write_file", "list_dir", "file_exists", "mkdir", "delete").
		AllowInState(agent.StateValidate, "read_file", "file_exists").
		Build()
}

type readFileInput struct {
	Path string `json:"path"`
}

type readFileOutput struct {
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

func readFileTool() tool.Tool {
	return tool.NewBuilder("read_file").
		WithDescription("Read contents of a file").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in readFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			content, err := os.ReadFile(in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			info, _ := os.Stat(in.Path)
			out := readFileOutput{
				Content: string(content),
				Size:    info.Size(),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type writeFileOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

func writeFileTool() tool.Tool {
	return tool.NewBuilder("write_file").
		WithDescription("Write content to a file").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in writeFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Ensure directory exists
			// Use 0750 for directories (owner: rwx, group: rx, others: none)
			dir := filepath.Dir(in.Path)
			if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301 -- intentionally restrictive
				return tool.Result{}, err
			}

			// Use 0600 for files (owner: rw, others: none)
			if err := os.WriteFile(in.Path, []byte(in.Content), 0600); err != nil { // #nosec G306 -- intentionally restrictive
				return tool.Result{}, err
			}

			out := writeFileOutput{
				Path:  in.Path,
				Bytes: len(in.Content),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type listDirInput struct {
	Path string `json:"path"`
}

type listDirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type listDirOutput struct {
	Entries []listDirEntry `json:"entries"`
	Count   int            `json:"count"`
}

func listDirTool() tool.Tool {
	return tool.NewBuilder("list_dir").
		WithDescription("List directory contents").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in listDirInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			entries, err := os.ReadDir(in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			out := listDirOutput{
				Entries: make([]listDirEntry, 0, len(entries)),
			}

			for _, e := range entries {
				info, _ := e.Info()
				var size int64
				if info != nil {
					size = info.Size()
				}
				out.Entries = append(out.Entries, listDirEntry{
					Name:  e.Name(),
					IsDir: e.IsDir(),
					Size:  size,
				})
			}
			out.Count = len(out.Entries)

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type fileExistsInput struct {
	Path string `json:"path"`
}

type fileExistsOutput struct {
	Exists bool  `json:"exists"`
	IsDir  bool  `json:"is_dir"`
	Size   int64 `json:"size,omitempty"`
}

func fileExistsTool() tool.Tool {
	return tool.NewBuilder("file_exists").
		WithDescription("Check if a file or directory exists").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in fileExistsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(in.Path)
			out := fileExistsOutput{
				Exists: err == nil,
			}

			if out.Exists {
				out.IsDir = info.IsDir()
				out.Size = info.Size()
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type mkdirInput struct {
	Path string `json:"path"`
}

type mkdirOutput struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

func mkdirTool() tool.Tool {
	return tool.NewBuilder("mkdir").
		WithDescription("Create a directory").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in mkdirInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Check if already exists
			_, err := os.Stat(in.Path)
			existed := err == nil

			// Use 0750 for directories (owner: rwx, group: rx, others: none)
			if err := os.MkdirAll(in.Path, 0750); err != nil { // #nosec G301 -- intentionally restrictive
				return tool.Result{}, err
			}

			out := mkdirOutput{
				Path:    in.Path,
				Created: !existed,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type deleteInput struct {
	Path string `json:"path"`
}

type deleteOutput struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("delete").
		WithDescription("Delete a file or directory").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Check if exists first
			_, err := os.Stat(in.Path)
			if os.IsNotExist(err) {
				out := deleteOutput{Path: in.Path, Deleted: false}
				data, _ := json.Marshal(out)
				return tool.Result{Output: data}, nil
			}

			if err := os.RemoveAll(in.Path); err != nil {
				return tool.Result{}, err
			}

			out := deleteOutput{Path: in.Path, Deleted: true}
			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
