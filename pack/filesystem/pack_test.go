package filesystem_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/pack/filesystem"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}

	tools := pack.Tools
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	// Verify expected tools (without delete by default)
	toolNames := make(map[string]bool)
	for _, tl := range tools {
		toolNames[tl.Name()] = true
	}

	// Default tools (fs_delete is only included when WithDeleteAccess is used)
	expectedTools := []string{"fs_read", "fs_write", "fs_list", "fs_watch"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestNewWithDeleteAccess(t *testing.T) {
	tmpDir := t.TempDir()

	pack, err := filesystem.New(
		filesystem.WithRootDir(tmpDir),
		filesystem.WithDeleteAccess(),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Verify delete tool is included
	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["fs_delete"] {
		t.Error("expected fs_delete tool when WithDeleteAccess is used")
	}
}

func TestFsRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_read tool
	var readTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_read" {
			readTool = tl
			break
		}
	}

	if readTool == nil {
		t.Fatal("fs_read tool not found")
	}

	// Execute read with absolute path
	input := json.RawMessage(`{"path": "` + testFile + `"}`)
	result, err := readTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("fs_read failed: %v", err)
	}

	// Verify output
	var output struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Content != content {
		t.Errorf("expected content %q, got %q", content, output.Content)
	}
}

func TestFsWrite(t *testing.T) {
	tmpDir := t.TempDir()

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_write tool
	var writeTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_write" {
			writeTool = tl
			break
		}
	}

	if writeTool == nil {
		t.Fatal("fs_write tool not found")
	}

	// Execute write with absolute path
	newFile := filepath.Join(tmpDir, "new.txt")
	input := json.RawMessage(`{"path": "` + newFile + `", "content": "new content"}`)
	_, err = writeTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("fs_write failed: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("expected content %q, got %q", "new content", string(content))
	}
}

func TestFsList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_list tool
	var listTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_list" {
			listTool = tl
			break
		}
	}

	if listTool == nil {
		t.Fatal("fs_list tool not found")
	}

	// Execute list with absolute path
	input := json.RawMessage(`{"path": "` + tmpDir + `"}`)
	result, err := listTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("fs_list failed: %v", err)
	}

	// Verify output contains files
	var output struct {
		Entries []struct {
			Name string `json:"name"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(output.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(output.Entries))
	}
}

func TestFsDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "delete_me.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pack, err := filesystem.New(
		filesystem.WithRootDir(tmpDir),
		filesystem.WithDeleteAccess(),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_delete tool
	var deleteTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_delete" {
			deleteTool = tl
			break
		}
	}

	if deleteTool == nil {
		t.Fatal("fs_delete tool not found")
	}

	// Execute delete with absolute path
	input := json.RawMessage(`{"path": "` + testFile + `"}`)
	_, err = deleteTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("fs_delete failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestFsDeleteDenied(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pack WITHOUT delete access (default)
	pack, err := filesystem.New(
		filesystem.WithRootDir(tmpDir),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// fs_delete tool should not be included when delete access is not enabled
	var deleteTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_delete" {
			deleteTool = tl
			break
		}
	}

	if deleteTool != nil {
		t.Fatal("fs_delete tool should not be present when WithDeleteAccess is not used")
	}
}

func TestFsReadOutsideRoot(t *testing.T) {
	tmpDir := t.TempDir()

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_read tool
	var readTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_read" {
			readTool = tl
			break
		}
	}

	// Try to read file outside root using path traversal
	input := json.RawMessage(`{"path": "` + filepath.Join(tmpDir, "..", "..", "etc", "passwd") + `"}`)
	_, err = readTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
}

func TestFsWatch(t *testing.T) {
	tmpDir := t.TempDir()

	pack, err := filesystem.New(filesystem.WithRootDir(tmpDir))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find fs_watch tool
	var watchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "fs_watch" {
			watchTool = tl
			break
		}
	}

	if watchTool == nil {
		t.Fatal("fs_watch tool not found")
	}

	// Test that watch tool exists and can be called
	// Use a very short duration to avoid blocking tests
	input := json.RawMessage(`{"path": "` + tmpDir + `", "duration_seconds": 1}`)
	result, err := watchTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("fs_watch failed: %v", err)
	}

	// Verify output structure
	var output struct {
		Path   string `json:"path"`
		Events []any  `json:"events"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Path != tmpDir {
		t.Errorf("expected path %s, got %s", tmpDir, output.Path)
	}
}
