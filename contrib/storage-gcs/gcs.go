// Package gcs provides Google Cloud Storage-backed implementations of agent-go storage interfaces.
//
// GCS is a scalable, fully managed object storage service that offers industry-leading
// durability and availability. It is ideal for storing large artifacts, backups, and
// any content that benefits from object storage semantics.
//
// # Usage
//
//	client, err := storage.NewClient(ctx)
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	store := gcs.NewArtifactStore(client, "my-bucket")
package gcs

import (
	"context"
	"io"

	"github.com/felixgeelhaar/agent-go/domain/artifact"
)

// Client represents a GCS client interface.
// This allows for mocking in tests.
type Client interface{}

// ArtifactStore is a GCS-backed implementation of artifact.Store.
// It stores artifacts as objects in a GCS bucket with metadata support.
type ArtifactStore struct {
	client     Client
	bucketName string
	prefix     string
}

// ArtifactStoreConfig holds configuration for the GCS artifact store.
type ArtifactStoreConfig struct {
	// BucketName is the GCS bucket name.
	BucketName string

	// Prefix is an optional prefix for all object keys.
	Prefix string

	// ComputeChecksum enables MD5 checksum computation on upload.
	ComputeChecksum bool
}

// NewArtifactStore creates a new GCS artifact store with the given client and bucket.
func NewArtifactStore(client Client, bucketName string) *ArtifactStore {
	return &ArtifactStore{
		client:     client,
		bucketName: bucketName,
	}
}

// NewArtifactStoreWithConfig creates a new GCS artifact store with full configuration.
func NewArtifactStoreWithConfig(client Client, cfg ArtifactStoreConfig) *ArtifactStore {
	return &ArtifactStore{
		client:     client,
		bucketName: cfg.BucketName,
		prefix:     cfg.Prefix,
	}
}

// Store saves content and returns a stable reference.
// The content is uploaded to GCS as an object with metadata stored as custom headers.
func (s *ArtifactStore) Store(ctx context.Context, content io.Reader, opts artifact.StoreOptions) (artifact.Ref, error) {
	// TODO: Implement GCS object upload
	// 1. Generate unique object name (UUID or content-addressable)
	// 2. Create object writer with content type and metadata
	// 3. Copy content to GCS
	// 4. Optionally compute checksum
	// 5. Return artifact reference with object details
	return artifact.Ref{}, nil
}

// Retrieve retrieves the content for an artifact reference.
// Returns an io.ReadCloser that must be closed by the caller.
func (s *ArtifactStore) Retrieve(ctx context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	// TODO: Implement GCS object download
	// 1. Get object handle from bucket
	// 2. Create reader
	// 3. Return reader (caller responsible for closing)
	return nil, nil
}

// Delete removes an artifact from the bucket.
func (s *ArtifactStore) Delete(ctx context.Context, ref artifact.Ref) error {
	// TODO: Implement GCS object deletion
	return nil
}

// Exists checks if an artifact exists in the bucket.
func (s *ArtifactStore) Exists(ctx context.Context, ref artifact.Ref) (bool, error) {
	// TODO: Implement GCS object existence check using Attrs
	return false, nil
}

// Metadata retrieves the metadata for an artifact without content.
// Returns a Ref populated with object attributes from GCS.
func (s *ArtifactStore) Metadata(ctx context.Context, ref artifact.Ref) (artifact.Ref, error) {
	// TODO: Implement GCS object metadata retrieval
	// 1. Get object handle
	// 2. Fetch Attrs
	// 3. Map to artifact.Ref (size, content type, metadata, etc.)
	return artifact.Ref{}, nil
}

// objectKey returns the full object key including prefix.
func (s *ArtifactStore) objectKey(id string) string {
	if s.prefix == "" {
		return id
	}
	return s.prefix + "/" + id
}

// Ensure interface is implemented.
var _ artifact.Store = (*ArtifactStore)(nil)
