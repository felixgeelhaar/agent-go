package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/sandbox"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack(t.TempDir())
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Metadata["status"] == "stub" {
		t.Fatal("expected no stub metadata")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "filesystem" {
		t.Errorf("expected pack name %q, got %q", "filesystem", p.Name)
	}
	if strings.Contains(p.Description, "STUB") {
		t.Errorf("description should not mention STUB: %q", p.Description)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack(t.TempDir())
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
		if tt.InputSchema().IsEmpty() {
			t.Errorf("tool %q has empty input schema", tt.Name())
		}
	}
}

func TestPack_ExpectedToolCount(t *testing.T) {
	p := Pack(t.TempDir())
	expected := 8
	if got := len(p.Tools); got != expected {
		t.Errorf("expected %d tools, got %d", expected, got)
	}
}

func getTool(t *testing.T, baseDir, name string) tool.Tool {
	t.Helper()
	p := Pack(baseDir)
	tt, ok := p.GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found in pack", name)
	}
	return tt
}

func execTool(t *testing.T, tt tool.Tool, input any) json.RawMessage {
	t.Helper()
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	result, err := tt.Execute(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("tool %q execution failed: %v", tt.Name(), err)
	}
	return result.Output
}

func TestFSReadWrite(t *testing.T) {
	baseDir := t.TempDir()
	writeTool := getTool(t, baseDir, "fs_write_file")
	readTool := getTool(t, baseDir, "fs_read_file")

	out := execTool(t, writeTool, map[string]any{"path": "hello.txt", "content": "hello world"})
	var writeResult writeFileOutput
	if err := json.Unmarshal(out, &writeResult); err != nil {
		t.Fatalf("unmarshal write output: %v", err)
	}
	if !writeResult.Created {
		t.Error("expected created to be true")
	}
	if writeResult.BytesWritten != 11 {
		t.Errorf("expected 11 bytes written, got %d", writeResult.BytesWritten)
	}

	out = execTool(t, readTool, map[string]any{"path": "hello.txt"})
	var readResult readFileOutput
	if err := json.Unmarshal(out, &readResult); err != nil {
		t.Fatalf("unmarshal read output: %v", err)
	}
	if readResult.Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", readResult.Content)
	}
	if readResult.Size != 11 {
		t.Errorf("expected size 11, got %d", readResult.Size)
	}
}

func TestFSListAndStat(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "a.txt"), []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(baseDir, "subdir"), 0750); err != nil {
		t.Fatal(err)
	}

	listTool := getTool(t, baseDir, "fs_list_dir")
	out := execTool(t, listTool, map[string]any{"path": "."})
	var listResult listDirOutput
	if err := json.Unmarshal(out, &listResult); err != nil {
		t.Fatalf("unmarshal list output: %v", err)
	}
	if listResult.Count != 2 {
		t.Errorf("expected 2 entries, got %d", listResult.Count)
	}

	statTool := getTool(t, baseDir, "fs_stat")
	out = execTool(t, statTool, map[string]any{"path": "a.txt"})
	var statResult statOutput
	if err := json.Unmarshal(out, &statResult); err != nil {
		t.Fatalf("unmarshal stat output: %v", err)
	}
	if statResult.Name != "a.txt" {
		t.Errorf("expected name a.txt, got %q", statResult.Name)
	}
	if statResult.Size != 3 {
		t.Errorf("expected size 3, got %d", statResult.Size)
	}
	if statResult.IsDir {
		t.Error("expected is_dir false for file")
	}
}

func TestFSMkdirCopyMoveRemove(t *testing.T) {
	baseDir := t.TempDir()

	mkdirTool := getTool(t, baseDir, "fs_mkdir")
	out := execTool(t, mkdirTool, map[string]any{"path": "nested/dir", "parents": true})
	var mkdirResult mkdirOutput
	if err := json.Unmarshal(out, &mkdirResult); err != nil {
		t.Fatalf("unmarshal mkdir output: %v", err)
	}
	if !mkdirResult.Created {
		t.Error("expected created to be true")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "nested", "dir")); err != nil {
		t.Fatalf("mkdir did not create directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "nested", "dir", "src.txt"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}

	copyTool := getTool(t, baseDir, "fs_copy")
	out = execTool(t, copyTool, map[string]any{"source": "nested/dir/src.txt", "dest": "nested/dir/copy.txt"})
	var copyResult copyOutput
	if err := json.Unmarshal(out, &copyResult); err != nil {
		t.Fatalf("unmarshal copy output: %v", err)
	}
	if copyResult.IsDir {
		t.Error("expected is_dir false for file copy")
	}
	data, err := os.ReadFile(filepath.Join(baseDir, "nested", "dir", "copy.txt"))
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != "payload" {
		t.Errorf("copied content mismatch: %q", string(data))
	}

	// Directory copy
	out = execTool(t, copyTool, map[string]any{"source": "nested/dir", "dest": "nested/dir2"})
	if err := json.Unmarshal(out, &copyResult); err != nil {
		t.Fatalf("unmarshal dir copy output: %v", err)
	}
	if !copyResult.IsDir {
		t.Error("expected is_dir true for directory copy")
	}

	moveTool := getTool(t, baseDir, "fs_move")
	out = execTool(t, moveTool, map[string]any{"source": "nested/dir/copy.txt", "dest": "nested/dir/moved.txt"})
	var moveResult moveOutput
	if err := json.Unmarshal(out, &moveResult); err != nil {
		t.Fatalf("unmarshal move output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "nested", "dir", "copy.txt")); !os.IsNotExist(err) {
		t.Error("expected source to be gone after move")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "nested", "dir", "moved.txt")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}

	removeTool := getTool(t, baseDir, "fs_remove")
	out = execTool(t, removeTool, map[string]any{"path": "nested/dir/moved.txt"})
	var removeResult removeOutput
	if err := json.Unmarshal(out, &removeResult); err != nil {
		t.Fatalf("unmarshal remove output: %v", err)
	}
	if !removeResult.Removed {
		t.Error("expected removed to be true")
	}

	out = execTool(t, removeTool, map[string]any{"path": "nested/dir2", "recursive": true})
	if err := json.Unmarshal(out, &removeResult); err != nil {
		t.Fatalf("unmarshal recursive remove output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "nested", "dir2")); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestPathTraversalRejected(t *testing.T) {
	baseDir := t.TempDir()
	tools := []struct {
		name  string
		input map[string]any
	}{
		{"fs_read_file", map[string]any{"path": "../../etc/passwd"}},
		{"fs_write_file", map[string]any{"path": "../escape.txt", "content": "x"}},
		{"fs_list_dir", map[string]any{"path": "../../"}},
		{"fs_stat", map[string]any{"path": "/etc/passwd"}},
		{"fs_mkdir", map[string]any{"path": "../outside"}},
		{"fs_remove", map[string]any{"path": "../../tmp"}},
		{"fs_copy", map[string]any{"source": "../../etc/passwd", "dest": "out.txt"}},
		{"fs_move", map[string]any{"source": "a.txt", "dest": "../escape.txt"}},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			tt := getTool(t, baseDir, tc.name)
			input, _ := json.Marshal(tc.input)
			_, err := tt.Execute(context.Background(), input)
			if err == nil {
				t.Error("expected error for path traversal")
			}
			if !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("expected path traversal error, got: %v", err)
			}
		})
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		path     string
		expected bool
	}{
		{"same dir", "/base", "/base", true},
		{"subdir", "/base", "/base/sub", true},
		{"parent escape", "/base", "/base/../etc", false},
		{"absolute escape", "/base", "/etc/passwd", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sandbox.IsUnderBase(tc.base, tc.path); got != tc.expected {
				t.Errorf("sandbox.IsUnderBase(%q, %q) = %v, want %v", tc.base, tc.path, got, tc.expected)
			}
		})
	}
}

func TestAnnotationsPreserved(t *testing.T) {
	p := Pack(t.TempDir())

	checks := map[string]func(tool.Annotations) bool{
		"fs_read_file":  func(a tool.Annotations) bool { return a.ReadOnly },
		"fs_list_dir":   func(a tool.Annotations) bool { return a.ReadOnly },
		"fs_stat":       func(a tool.Annotations) bool { return a.ReadOnly },
		"fs_write_file": func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskMedium },
		"fs_mkdir":      func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"fs_remove":     func(a tool.Annotations) bool { return a.Destructive },
		"fs_copy":       func(a tool.Annotations) bool { return a.RiskLevel == tool.RiskMedium },
		"fs_move":       func(a tool.Annotations) bool { return a.RiskLevel == tool.RiskMedium },
	}

	for name, check := range checks {
		tt, ok := p.GetTool(name)
		if !ok {
			t.Fatalf("tool %q not found", name)
		}
		if !check(tt.Annotations()) {
			t.Errorf("tool %q annotations not preserved: %+v", name, tt.Annotations())
		}
	}
}
