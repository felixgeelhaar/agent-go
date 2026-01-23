package gcs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/artifact"
)

func TestNewArtifactStore(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Client: NewMockClient(),
				Bucket: "test-bucket",
				Prefix: "artifacts",
			},
			wantErr: false,
		},
		{
			name: "missing client",
			cfg: Config{
				Bucket: "test-bucket",
			},
			wantErr: true,
		},
		{
			name: "missing bucket",
			cfg: Config{
				Client: NewMockClient(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArtifactStore(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewArtifactStore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
	})

	tests := []struct {
		name    string
		content string
		opts    artifact.StoreOptions
		wantErr bool
	}{
		{
			name:    "store simple content",
			content: "Hello, World!",
			opts:    artifact.DefaultStoreOptions(),
			wantErr: false,
		},
		{
			name:    "store with name",
			content: "Named content",
			opts:    artifact.DefaultStoreOptions().WithName("test-artifact"),
			wantErr: false,
		},
		{
			name:    "store with metadata",
			content: "Content with metadata",
			opts:    artifact.DefaultStoreOptions().WithMetadata("key", "value"),
			wantErr: false,
		},
		{
			name:    "store with content type",
			content: `{"key": "value"}`,
			opts:    artifact.DefaultStoreOptions().WithContentType("application/json"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tt.content))
			ref, err := store.Store(ctx, reader, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !ref.IsValid() {
				t.Error("expected valid ref")
			}

			if ref.Size != int64(len(tt.content)) {
				t.Errorf("expected size %d, got %d", len(tt.content), ref.Size)
			}

			if tt.opts.Name != "" && ref.Name != tt.opts.Name {
				t.Errorf("expected name %q, got %q", tt.opts.Name, ref.Name)
			}

			if tt.opts.ComputeChecksum && ref.Checksum == "" {
				t.Error("expected checksum to be computed")
			}
		})
	}
}

func TestRetrieve(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
	})

	// Store content first
	content := "Test content for retrieval"
	reader := bytes.NewReader([]byte(content))
	ref, _ := store.Store(ctx, reader, artifact.DefaultStoreOptions())

	tests := []struct {
		name    string
		ref     artifact.Ref
		wantErr error
	}{
		{
			name:    "retrieve existing",
			ref:     ref,
			wantErr: nil,
		},
		{
			name:    "retrieve non-existent",
			ref:     artifact.NewRef("non-existent"),
			wantErr: artifact.ErrArtifactNotFound,
		},
		{
			name:    "retrieve invalid ref",
			ref:     artifact.Ref{},
			wantErr: artifact.ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := store.Retrieve(ctx, tt.ref)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			defer reader.Close()
			data, _ := io.ReadAll(reader)
			if string(data) != content {
				t.Errorf("expected content %q, got %q", content, string(data))
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
	})

	// Store content first
	content := "Content to delete"
	reader := bytes.NewReader([]byte(content))
	ref, _ := store.Store(ctx, reader, artifact.DefaultStoreOptions())

	tests := []struct {
		name    string
		ref     artifact.Ref
		wantErr error
	}{
		{
			name:    "delete existing",
			ref:     ref,
			wantErr: nil,
		},
		{
			name:    "delete non-existent",
			ref:     artifact.NewRef("non-existent"),
			wantErr: artifact.ErrArtifactNotFound,
		},
		{
			name:    "delete invalid ref",
			ref:     artifact.Ref{},
			wantErr: artifact.ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(ctx, tt.ref)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify deletion
			exists, _ := store.Exists(ctx, tt.ref)
			if exists {
				t.Error("expected artifact to be deleted")
			}
		})
	}
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
	})

	// Store content first
	content := "Content to check"
	reader := bytes.NewReader([]byte(content))
	ref, _ := store.Store(ctx, reader, artifact.DefaultStoreOptions())

	tests := []struct {
		name       string
		ref        artifact.Ref
		wantExists bool
		wantErr    error
	}{
		{
			name:       "exists",
			ref:        ref,
			wantExists: true,
			wantErr:    nil,
		},
		{
			name:       "does not exist",
			ref:        artifact.NewRef("non-existent"),
			wantExists: false,
			wantErr:    nil,
		},
		{
			name:       "invalid ref",
			ref:        artifact.Ref{},
			wantExists: false,
			wantErr:    artifact.ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := store.Exists(ctx, tt.ref)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if exists != tt.wantExists {
				t.Errorf("expected exists %v, got %v", tt.wantExists, exists)
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
	})

	// Store content with metadata
	content := "Content with metadata"
	reader := bytes.NewReader([]byte(content))
	opts := artifact.DefaultStoreOptions().
		WithName("test-artifact").
		WithContentType("text/plain").
		WithMetadata("key", "value")
	ref, _ := store.Store(ctx, reader, opts)

	tests := []struct {
		name    string
		ref     artifact.Ref
		wantErr error
	}{
		{
			name:    "get metadata",
			ref:     ref,
			wantErr: nil,
		},
		{
			name:    "metadata for non-existent",
			ref:     artifact.NewRef("non-existent"),
			wantErr: artifact.ErrArtifactNotFound,
		},
		{
			name:    "metadata for invalid ref",
			ref:     artifact.Ref{},
			wantErr: artifact.ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := store.Metadata(ctx, tt.ref)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if meta.ID != ref.ID {
				t.Errorf("expected ID %q, got %q", ref.ID, meta.ID)
			}

			if meta.Name != "test-artifact" {
				t.Errorf("expected name %q, got %q", "test-artifact", meta.Name)
			}

			if meta.ContentType != "text/plain" {
				t.Errorf("expected content type %q, got %q", "text/plain", meta.ContentType)
			}

			if meta.Metadata["key"] != "value" {
				t.Errorf("expected metadata key %q, got %q", "value", meta.Metadata["key"])
			}
		})
	}
}

func TestWithPrefix(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewArtifactStore(Config{
		Client: client,
		Bucket: "test-bucket",
		Prefix: "artifacts/v1",
	})

	content := "Prefixed content"
	reader := bytes.NewReader([]byte(content))
	ref, err := store.Store(ctx, reader, artifact.DefaultStoreOptions())
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Verify we can retrieve it
	retrieved, err := store.Retrieve(ctx, ref)
	if err != nil {
		t.Fatalf("failed to retrieve: %v", err)
	}
	defer retrieved.Close()

	data, _ := io.ReadAll(retrieved)
	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestMockClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()

	// Test Upload
	content := []byte("test content")
	err := client.Upload(ctx, "bucket", "object", bytes.NewReader(content))
	if err != nil {
		t.Errorf("Upload error: %v", err)
	}

	// Test Exists
	exists, err := client.Exists(ctx, "bucket", "object")
	if err != nil {
		t.Errorf("Exists error: %v", err)
	}
	if !exists {
		t.Error("expected object to exist")
	}

	// Test Download
	reader, err := client.Download(ctx, "bucket", "object")
	if err != nil {
		t.Errorf("Download error: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if !bytes.Equal(data, content) {
		t.Errorf("expected content %q, got %q", content, data)
	}

	// Test Delete
	err = client.Delete(ctx, "bucket", "object")
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}

	// Verify deletion
	exists, _ = client.Exists(ctx, "bucket", "object")
	if exists {
		t.Error("expected object to be deleted")
	}

	// Test Download non-existent
	_, err = client.Download(ctx, "bucket", "non-existent")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}

	// Test ObjectCount
	_ = client.Upload(ctx, "bucket", "obj1", bytes.NewReader([]byte("1")))
	_ = client.Upload(ctx, "bucket", "obj2", bytes.NewReader([]byte("2")))
	if client.ObjectCount() != 2 {
		t.Errorf("expected 2 objects, got %d", client.ObjectCount())
	}
}
