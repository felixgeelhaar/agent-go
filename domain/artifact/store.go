package artifact

import (
	"context"
	"errors"
	"io"
)

// Store defines the interface for artifact storage.
// Implementations are in infrastructure.
type Store interface {
	// Store saves content and returns a stable reference.
	Store(ctx context.Context, content io.Reader, opts StoreOptions) (Ref, error)

	// Retrieve retrieves the content for an artifact reference.
	Retrieve(ctx context.Context, ref Ref) (io.ReadCloser, error)

	// Delete removes an artifact.
	Delete(ctx context.Context, ref Ref) error

	// Exists checks if an artifact exists.
	Exists(ctx context.Context, ref Ref) (bool, error)

	// Metadata retrieves the metadata for an artifact without content.
	Metadata(ctx context.Context, ref Ref) (Ref, error)
}

// StoreOptions configures artifact storage.
type StoreOptions struct {
	// Name is an optional human-readable name.
	Name string

	// ContentType is the MIME type of the content.
	ContentType string

	// Metadata contains arbitrary key-value pairs.
	Metadata map[string]string

	// ComputeChecksum enables checksum computation.
	ComputeChecksum bool
}

// DefaultStoreOptions returns options with sensible defaults.
func DefaultStoreOptions() StoreOptions {
	return StoreOptions{
		ContentType:     "application/octet-stream",
		ComputeChecksum: true,
	}
}

// WithName sets the artifact name.
func (o StoreOptions) WithName(name string) StoreOptions {
	o.Name = name
	return o
}

// WithContentType sets the content type.
func (o StoreOptions) WithContentType(contentType string) StoreOptions {
	o.ContentType = contentType
	return o
}

// WithMetadata adds metadata.
func (o StoreOptions) WithMetadata(key, value string) StoreOptions {
	if o.Metadata == nil {
		o.Metadata = make(map[string]string)
	}
	o.Metadata[key] = value
	return o
}

// Domain errors for artifact storage.
var (
	// ErrArtifactNotFound indicates the artifact was not found.
	ErrArtifactNotFound = errors.New("artifact not found")

	// ErrArtifactExists indicates an artifact with the same ID exists.
	ErrArtifactExists = errors.New("artifact already exists")

	// ErrInvalidRef indicates the artifact reference is invalid.
	ErrInvalidRef = errors.New("invalid artifact reference")

	// ErrStoreFull indicates the store has reached capacity.
	ErrStoreFull = errors.New("artifact store full")

	// ErrChecksumMismatch indicates the content checksum doesn't match.
	ErrChecksumMismatch = errors.New("checksum mismatch")
)
