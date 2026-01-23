// Package gcs provides Google Cloud Storage implementations.
package gcs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/artifact"
)

// Client defines the interface for GCS client operations.
// This allows for mock implementations in testing.
type Client interface {
	// Upload uploads content to a bucket.
	Upload(ctx context.Context, bucket, object string, content io.Reader) error

	// Download downloads content from a bucket.
	Download(ctx context.Context, bucket, object string) (io.ReadCloser, error)

	// Delete deletes an object from a bucket.
	Delete(ctx context.Context, bucket, object string) error

	// Exists checks if an object exists.
	Exists(ctx context.Context, bucket, object string) (bool, error)
}

// ArtifactStore implements artifact.Store using Google Cloud Storage.
type ArtifactStore struct {
	client     Client
	bucket     string
	prefix     string
}

// Config holds configuration for the GCS artifact store.
type Config struct {
	// Client is the GCS client to use.
	Client Client

	// Bucket is the GCS bucket name.
	Bucket string

	// Prefix is an optional prefix for all objects.
	Prefix string
}

// NewArtifactStore creates a new GCS artifact store.
func NewArtifactStore(cfg Config) (*ArtifactStore, error) {
	if cfg.Client == nil {
		return nil, errors.New("gcs client is required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("bucket name is required")
	}

	return &ArtifactStore{
		client: cfg.Client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// Store saves content and returns a stable reference.
func (s *ArtifactStore) Store(ctx context.Context, content io.Reader, opts artifact.StoreOptions) (artifact.Ref, error) {
	// Generate unique ID
	id := generateArtifactID()

	// Read content into buffer for checksum and size
	var buf bytes.Buffer
	hasher := sha256.New()
	writer := io.MultiWriter(&buf, hasher)

	size, err := io.Copy(writer, content)
	if err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to read content: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	// Upload content
	contentPath := s.objectPath(id, "content")
	if err := s.client.Upload(ctx, s.bucket, contentPath, &buf); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to upload content: %w", err)
	}

	// Create reference
	ref := artifact.NewRef(id).
		WithSize(size).
		WithContentType(opts.ContentType)

	if opts.Name != "" {
		ref = ref.WithName(opts.Name)
	}

	if opts.ComputeChecksum {
		ref = ref.WithChecksum(checksum)
	}

	for k, v := range opts.Metadata {
		ref = ref.WithMetadata(k, v)
	}

	// Upload metadata
	metaPath := s.objectPath(id, "metadata.json")
	metaData, err := json.Marshal(ref)
	if err != nil {
		// Cleanup content on metadata failure
		_ = s.client.Delete(ctx, s.bucket, contentPath)
		return artifact.Ref{}, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := s.client.Upload(ctx, s.bucket, metaPath, bytes.NewReader(metaData)); err != nil {
		// Cleanup content on metadata upload failure
		_ = s.client.Delete(ctx, s.bucket, contentPath)
		return artifact.Ref{}, fmt.Errorf("failed to upload metadata: %w", err)
	}

	return ref, nil
}

// Retrieve retrieves the content for an artifact reference.
func (s *ArtifactStore) Retrieve(ctx context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	if !ref.IsValid() {
		return nil, artifact.ErrInvalidRef
	}

	contentPath := s.objectPath(ref.ID, "content")
	reader, err := s.client.Download(ctx, s.bucket, contentPath)
	if err != nil {
		// Check if object not found
		if isNotFoundError(err) {
			return nil, artifact.ErrArtifactNotFound
		}
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	return reader, nil
}

// Delete removes an artifact.
func (s *ArtifactStore) Delete(ctx context.Context, ref artifact.Ref) error {
	if !ref.IsValid() {
		return artifact.ErrInvalidRef
	}

	contentPath := s.objectPath(ref.ID, "content")
	metaPath := s.objectPath(ref.ID, "metadata.json")

	// Check if artifact exists
	exists, err := s.client.Exists(ctx, s.bucket, contentPath)
	if err != nil {
		return fmt.Errorf("failed to check artifact existence: %w", err)
	}
	if !exists {
		return artifact.ErrArtifactNotFound
	}

	// Delete content
	if err := s.client.Delete(ctx, s.bucket, contentPath); err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	// Delete metadata (best effort)
	_ = s.client.Delete(ctx, s.bucket, metaPath)

	return nil
}

// Exists checks if an artifact exists.
func (s *ArtifactStore) Exists(ctx context.Context, ref artifact.Ref) (bool, error) {
	if !ref.IsValid() {
		return false, artifact.ErrInvalidRef
	}

	contentPath := s.objectPath(ref.ID, "content")
	return s.client.Exists(ctx, s.bucket, contentPath)
}

// Metadata retrieves the metadata for an artifact without content.
func (s *ArtifactStore) Metadata(ctx context.Context, ref artifact.Ref) (artifact.Ref, error) {
	if !ref.IsValid() {
		return artifact.Ref{}, artifact.ErrInvalidRef
	}

	metaPath := s.objectPath(ref.ID, "metadata.json")
	reader, err := s.client.Download(ctx, s.bucket, metaPath)
	if err != nil {
		if isNotFoundError(err) {
			return artifact.Ref{}, artifact.ErrArtifactNotFound
		}
		return artifact.Ref{}, fmt.Errorf("failed to download metadata: %w", err)
	}
	defer reader.Close()

	var storedRef artifact.Ref
	if err := json.NewDecoder(reader).Decode(&storedRef); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return storedRef, nil
}

// objectPath constructs the full object path.
func (s *ArtifactStore) objectPath(id, name string) string {
	if s.prefix != "" {
		return s.prefix + "/" + id + "/" + name
	}
	return id + "/" + name
}

// generateArtifactID creates a unique artifact ID.
func generateArtifactID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString generates a random alphanumeric string.
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond) // Ensure uniqueness
	}
	return string(b)
}

// isNotFoundError checks if an error indicates object not found.
func isNotFoundError(err error) bool {
	return errors.Is(err, ErrObjectNotFound)
}

// ErrObjectNotFound is returned when an object is not found.
var ErrObjectNotFound = errors.New("object not found")

// Ensure ArtifactStore implements artifact.Store
var _ artifact.Store = (*ArtifactStore)(nil)

// MockClient is a mock GCS client for testing.
type MockClient struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMockClient creates a new mock GCS client.
func NewMockClient() *MockClient {
	return &MockClient{
		objects: make(map[string][]byte),
	}
}

// Upload implements Client.Upload.
func (c *MockClient) Upload(_ context.Context, bucket, object string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := bucket + "/" + object
	c.objects[key] = data
	return nil
}

// Download implements Client.Download.
func (c *MockClient) Download(_ context.Context, bucket, object string) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := bucket + "/" + object
	data, ok := c.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

// Delete implements Client.Delete.
func (c *MockClient) Delete(_ context.Context, bucket, object string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := bucket + "/" + object
	delete(c.objects, key)
	return nil
}

// Exists implements Client.Exists.
func (c *MockClient) Exists(_ context.Context, bucket, object string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := bucket + "/" + object
	_, ok := c.objects[key]
	return ok, nil
}

// ObjectCount returns the number of objects (for testing).
func (c *MockClient) ObjectCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.objects)
}

// Ensure MockClient implements Client
var _ Client = (*MockClient)(nil)
