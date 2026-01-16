package filesystem

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/artifact"
)

func TestNewArtifactStore(t *testing.T) {
	t.Parallel()

	t.Run("creates store in existing directory", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, err := NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("NewArtifactStore() error = %v", err)
		}
		if store == nil {
			t.Fatal("NewArtifactStore() returned nil")
		}
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		newPath := filepath.Join(tempDir, "new", "nested", "dir")

		store, err := NewArtifactStore(newPath)
		if err != nil {
			t.Fatalf("NewArtifactStore() error = %v", err)
		}
		if store == nil {
			t.Fatal("NewArtifactStore() returned nil")
		}

		// Verify directory was created
		info, err := os.Stat(newPath)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected a directory")
		}
	})
}

func TestArtifactStore_Store(t *testing.T) {
	t.Parallel()

	t.Run("stores content successfully", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		content := "Hello, World!"
		reader := strings.NewReader(content)

		opts := artifact.DefaultStoreOptions().
			WithName("test.txt").
			WithContentType("text/plain")

		ref, err := store.Store(ctx, reader, opts)
		if err != nil {
			t.Fatalf("Store() error = %v", err)
		}

		if ref.ID == "" {
			t.Error("expected non-empty ID")
		}
		if ref.Name != "test.txt" {
			t.Errorf("Name = %s, want test.txt", ref.Name)
		}
		if ref.ContentType != "text/plain" {
			t.Errorf("ContentType = %s, want text/plain", ref.ContentType)
		}
		if ref.Size != int64(len(content)) {
			t.Errorf("Size = %d, want %d", ref.Size, len(content))
		}
		if ref.Checksum == "" {
			t.Error("expected checksum to be computed")
		}
	})

	t.Run("stores with metadata", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		opts := artifact.StoreOptions{
			Name:        "test.txt",
			ContentType: "text/plain",
			Metadata: map[string]string{
				"author":  "test",
				"version": "1.0",
			},
			ComputeChecksum: true,
		}

		ref, err := store.Store(ctx, strings.NewReader("content"), opts)
		if err != nil {
			t.Fatalf("Store() error = %v", err)
		}

		if ref.Metadata["author"] != "test" {
			t.Errorf("Metadata[author] = %s, want test", ref.Metadata["author"])
		}
		if ref.Metadata["version"] != "1.0" {
			t.Errorf("Metadata[version] = %s, want 1.0", ref.Metadata["version"])
		}
	})

	t.Run("stores without checksum computation", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		opts := artifact.StoreOptions{
			ComputeChecksum: false,
		}

		ref, err := store.Store(ctx, strings.NewReader("content"), opts)
		if err != nil {
			t.Fatalf("Store() error = %v", err)
		}

		if ref.Checksum != "" {
			t.Error("expected empty checksum when ComputeChecksum is false")
		}
	})

	t.Run("stores binary content", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
		reader := bytes.NewReader(binaryData)

		opts := artifact.DefaultStoreOptions().
			WithContentType("application/octet-stream")

		ref, err := store.Store(ctx, reader, opts)
		if err != nil {
			t.Fatalf("Store() error = %v", err)
		}

		if ref.Size != int64(len(binaryData)) {
			t.Errorf("Size = %d, want %d", ref.Size, len(binaryData))
		}
	})
}

func TestArtifactStore_Retrieve(t *testing.T) {
	t.Parallel()

	t.Run("retrieves stored content", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		content := "Test content for retrieval"
		ref, _ := store.Store(ctx, strings.NewReader(content), artifact.DefaultStoreOptions())

		reader, err := store.Retrieve(ctx, ref)
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if string(data) != content {
			t.Errorf("content = %s, want %s", string(data), content)
		}
	})

	t.Run("returns error for invalid ref", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		invalidRef := artifact.Ref{ID: ""} // Invalid - empty ID
		_, err := store.Retrieve(ctx, invalidRef)
		if err != artifact.ErrInvalidRef {
			t.Errorf("Retrieve() error = %v, want ErrInvalidRef", err)
		}
	})

	t.Run("returns error for non-existent artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		nonExistentRef := artifact.NewRef("non-existent-id")
		_, err := store.Retrieve(ctx, nonExistentRef)
		if err != artifact.ErrArtifactNotFound {
			t.Errorf("Retrieve() error = %v, want ErrArtifactNotFound", err)
		}
	})
}

func TestArtifactStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		ref, _ := store.Store(ctx, strings.NewReader("content"), artifact.DefaultStoreOptions())

		err := store.Delete(ctx, ref)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify artifact is deleted
		exists, _ := store.Exists(ctx, ref)
		if exists {
			t.Error("expected artifact to be deleted")
		}
	})

	t.Run("returns error for invalid ref", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		invalidRef := artifact.Ref{ID: ""}
		err := store.Delete(ctx, invalidRef)
		if err != artifact.ErrInvalidRef {
			t.Errorf("Delete() error = %v, want ErrInvalidRef", err)
		}
	})

	t.Run("returns error for non-existent artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		nonExistentRef := artifact.NewRef("non-existent-id")
		err := store.Delete(ctx, nonExistentRef)
		if err != artifact.ErrArtifactNotFound {
			t.Errorf("Delete() error = %v, want ErrArtifactNotFound", err)
		}
	})
}

func TestArtifactStore_Exists(t *testing.T) {
	t.Parallel()

	t.Run("returns true for existing artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		ref, _ := store.Store(ctx, strings.NewReader("content"), artifact.DefaultStoreOptions())

		exists, err := store.Exists(ctx, ref)
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !exists {
			t.Error("expected Exists = true")
		}
	})

	t.Run("returns false for non-existent artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		nonExistentRef := artifact.NewRef("non-existent-id")
		exists, err := store.Exists(ctx, nonExistentRef)
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if exists {
			t.Error("expected Exists = false")
		}
	})

	t.Run("returns error for invalid ref", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		invalidRef := artifact.Ref{ID: ""}
		_, err := store.Exists(ctx, invalidRef)
		if err != artifact.ErrInvalidRef {
			t.Errorf("Exists() error = %v, want ErrInvalidRef", err)
		}
	})
}

func TestArtifactStore_Metadata(t *testing.T) {
	t.Parallel()

	t.Run("retrieves metadata for existing artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		opts := artifact.StoreOptions{
			Name:        "test.txt",
			ContentType: "text/plain",
			Metadata: map[string]string{
				"author": "tester",
			},
			ComputeChecksum: true,
		}

		storedRef, _ := store.Store(ctx, strings.NewReader("content"), opts)

		retrievedRef, err := store.Metadata(ctx, storedRef)
		if err != nil {
			t.Fatalf("Metadata() error = %v", err)
		}

		if retrievedRef.ID != storedRef.ID {
			t.Errorf("ID = %s, want %s", retrievedRef.ID, storedRef.ID)
		}
		if retrievedRef.Name != "test.txt" {
			t.Errorf("Name = %s, want test.txt", retrievedRef.Name)
		}
		if retrievedRef.ContentType != "text/plain" {
			t.Errorf("ContentType = %s, want text/plain", retrievedRef.ContentType)
		}
		if retrievedRef.Metadata["author"] != "tester" {
			t.Errorf("Metadata[author] = %s, want tester", retrievedRef.Metadata["author"])
		}
	})

	t.Run("returns error for invalid ref", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		invalidRef := artifact.Ref{ID: ""}
		_, err := store.Metadata(ctx, invalidRef)
		if err != artifact.ErrInvalidRef {
			t.Errorf("Metadata() error = %v, want ErrInvalidRef", err)
		}
	})

	t.Run("returns error for non-existent artifact", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		store, _ := NewArtifactStore(tempDir)
		ctx := context.Background()

		nonExistentRef := artifact.NewRef("non-existent-id")
		_, err := store.Metadata(ctx, nonExistentRef)
		if err != artifact.ErrArtifactNotFound {
			t.Errorf("Metadata() error = %v, want ErrArtifactNotFound", err)
		}
	})
}

func TestArtifactStore_Roundtrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, _ := NewArtifactStore(tempDir)
	ctx := context.Background()

	// Store
	content := "Complete roundtrip test content"
	opts := artifact.DefaultStoreOptions().
		WithName("roundtrip.txt").
		WithContentType("text/plain").
		WithMetadata("test", "value")

	ref, err := store.Store(ctx, strings.NewReader(content), opts)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Exists
	exists, err := store.Exists(ctx, ref)
	if err != nil || !exists {
		t.Fatalf("Exists() = %v, error = %v, want true, nil", exists, err)
	}

	// Retrieve
	reader, err := store.Retrieve(ctx, ref)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	data, _ := io.ReadAll(reader)
	reader.Close()
	if string(data) != content {
		t.Errorf("retrieved content = %s, want %s", string(data), content)
	}

	// Metadata
	meta, err := store.Metadata(ctx, ref)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if meta.Name != "roundtrip.txt" {
		t.Errorf("Metadata.Name = %s, want roundtrip.txt", meta.Name)
	}

	// Delete
	err = store.Delete(ctx, ref)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	exists, _ = store.Exists(ctx, ref)
	if exists {
		t.Error("expected artifact to be deleted")
	}
}

func TestGenerateArtifactID(t *testing.T) {
	t.Parallel()

	id1 := generateArtifactID()
	id2 := generateArtifactID()

	if id1 == "" {
		t.Error("generateArtifactID() returned empty string")
	}
	if id2 == "" {
		t.Error("generateArtifactID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateArtifactID() should return unique IDs")
	}
}

func TestRandomString(t *testing.T) {
	t.Parallel()

	s := randomString(8)
	if len(s) != 8 {
		t.Errorf("randomString(8) returned string of length %d, want 8", len(s))
	}

	// Verify it only contains valid characters
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, c := range s {
		if !strings.ContainsRune(charset, c) {
			t.Errorf("randomString() returned invalid character: %c", c)
		}
	}
}
