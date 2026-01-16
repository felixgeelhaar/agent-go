package fileops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}

	if p.Name != "fileops" {
		t.Errorf("Name = %s, want fileops", p.Name)
	}

	// Check that expected tools exist
	expectedTools := []string{"read_file", "write_file", "list_dir", "file_exists", "mkdir", "delete"}
	for _, name := range expectedTools {
		if _, ok := p.GetTool(name); !ok {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestReadFileTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("read_file")
	if !ok {
		t.Fatal("read_file tool not found")
	}

	// Create temp file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input, _ := json.Marshal(readFileInput{Path: testFile})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out readFileOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Content != content {
		t.Errorf("Content = %s, want %s", out.Content, content)
	}
	if out.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", out.Size, len(content))
	}
}

func TestReadFileToolNotFound(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("read_file")

	input, _ := json.Marshal(readFileInput{Path: "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFileToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("read_file")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWriteFileTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("write_file")
	if !ok {
		t.Fatal("write_file tool not found")
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "output.txt")
	content := "Test content"

	input, _ := json.Marshal(writeFileInput{
		Path:    testFile,
		Content: content,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out writeFileOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Path != testFile {
		t.Errorf("Path = %s, want %s", out.Path, testFile)
	}
	if out.Bytes != len(content) {
		t.Errorf("Bytes = %d, want %d", out.Bytes, len(content))
	}

	// Verify file was written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %s, want %s", string(data), content)
	}
}

func TestWriteFileToolCreatesDir(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("write_file")

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "subdir", "nested", "output.txt")

	input, _ := json.Marshal(writeFileInput{
		Path:    testFile,
		Content: "nested content",
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify file was created in nested directory
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestWriteFileToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("write_file")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestListDirTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("list_dir")
	if !ok {
		t.Fatal("list_dir tool not found")
	}

	tempDir := t.TempDir()
	// Create some files and a subdirectory
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)

	input, _ := json.Marshal(listDirInput{Path: tempDir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out listDirOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 3 {
		t.Errorf("Count = %d, want 3", out.Count)
	}

	// Check that we have correct entries
	hasSubdir := false
	for _, entry := range out.Entries {
		if entry.Name == "subdir" && entry.IsDir {
			hasSubdir = true
		}
	}
	if !hasSubdir {
		t.Error("expected subdir entry")
	}
}

func TestListDirToolNotFound(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("list_dir")

	input, _ := json.Marshal(listDirInput{Path: "/nonexistent/directory"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestListDirToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("list_dir")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFileExistsTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("file_exists")
	if !ok {
		t.Fatal("file_exists tool not found")
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "exists.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	t.Run("file exists", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(fileExistsInput{Path: testFile})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out fileExistsOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if !out.Exists {
			t.Error("expected Exists = true")
		}
		if out.IsDir {
			t.Error("expected IsDir = false")
		}
	})

	t.Run("directory exists", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(fileExistsInput{Path: tempDir})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out fileExistsOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if !out.Exists {
			t.Error("expected Exists = true")
		}
		if !out.IsDir {
			t.Error("expected IsDir = true")
		}
	})

	t.Run("does not exist", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(fileExistsInput{Path: "/nonexistent/file.txt"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out fileExistsOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if out.Exists {
			t.Error("expected Exists = false")
		}
	})
}

func TestFileExistsToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("file_exists")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMkdirTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("mkdir")
	if !ok {
		t.Fatal("mkdir tool not found")
	}

	tempDir := t.TempDir()

	t.Run("creates new directory", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "newdir")
		input, _ := json.Marshal(mkdirInput{Path: newDir})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out mkdirOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if !out.Created {
			t.Error("expected Created = true")
		}

		// Verify directory exists
		info, err := os.Stat(newDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected a directory")
		}
	})

	t.Run("idempotent for existing directory", func(t *testing.T) {
		existingDir := filepath.Join(tempDir, "existing")
		os.Mkdir(existingDir, 0755)

		input, _ := json.Marshal(mkdirInput{Path: existingDir})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out mkdirOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if out.Created {
			t.Error("expected Created = false for existing directory")
		}
	})
}

func TestMkdirToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("mkdir")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDeleteTool(t *testing.T) {
	t.Parallel()

	p := New()
	tool, ok := p.GetTool("delete")
	if !ok {
		t.Fatal("delete tool not found")
	}

	t.Run("deletes existing file", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "to_delete.txt")
		os.WriteFile(testFile, []byte("content"), 0644)

		input, _ := json.Marshal(deleteInput{Path: testFile})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out deleteOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if !out.Deleted {
			t.Error("expected Deleted = true")
		}

		// Verify file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("expected file to be deleted")
		}
	})

	t.Run("handles non-existent file", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(deleteInput{Path: "/nonexistent/file.txt"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out deleteOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if out.Deleted {
			t.Error("expected Deleted = false for non-existent file")
		}
	})

	t.Run("deletes directory recursively", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		dirToDelete := filepath.Join(tempDir, "dir_to_delete")
		os.MkdirAll(filepath.Join(dirToDelete, "subdir"), 0755)
		os.WriteFile(filepath.Join(dirToDelete, "file.txt"), []byte("content"), 0644)

		input, _ := json.Marshal(deleteInput{Path: dirToDelete})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var out deleteOutput
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}

		if !out.Deleted {
			t.Error("expected Deleted = true")
		}

		// Verify directory is deleted
		if _, err := os.Stat(dirToDelete); !os.IsNotExist(err) {
			t.Error("expected directory to be deleted")
		}
	})
}

func TestDeleteToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("delete")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestToolAnnotations(t *testing.T) {
	t.Parallel()

	p := New()

	// read_file should be read-only and cacheable
	if tool, ok := p.GetTool("read_file"); ok {
		a := tool.Annotations()
		if !a.ReadOnly {
			t.Error("read_file should be ReadOnly")
		}
		if !a.Cacheable {
			t.Error("read_file should be Cacheable")
		}
	}

	// write_file should be destructive
	if tool, ok := p.GetTool("write_file"); ok {
		a := tool.Annotations()
		if !a.Destructive {
			t.Error("write_file should be Destructive")
		}
	}

	// mkdir should be idempotent
	if tool, ok := p.GetTool("mkdir"); ok {
		a := tool.Annotations()
		if !a.Idempotent {
			t.Error("mkdir should be Idempotent")
		}
	}

	// delete should be destructive
	if tool, ok := p.GetTool("delete"); ok {
		a := tool.Annotations()
		if !a.Destructive {
			t.Error("delete should be Destructive")
		}
	}
}
