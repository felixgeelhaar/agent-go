// Package artifact provides domain models for artifact storage.
package artifact

import (
	"time"
)

// Ref is a stable reference to a stored artifact.
type Ref struct {
	// ID is the unique identifier for the artifact.
	ID string `json:"id"`

	// Name is an optional human-readable name.
	Name string `json:"name,omitempty"`

	// ContentType is the MIME type of the artifact.
	ContentType string `json:"content_type,omitempty"`

	// Size is the size of the artifact in bytes.
	Size int64 `json:"size"`

	// Checksum is the content hash for integrity verification.
	Checksum string `json:"checksum,omitempty"`

	// CreatedAt is when the artifact was stored.
	CreatedAt time.Time `json:"created_at"`

	// Metadata contains arbitrary key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewRef creates a new artifact reference.
func NewRef(id string) Ref {
	return Ref{
		ID:        id,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// WithName sets the artifact name.
func (r Ref) WithName(name string) Ref {
	r.Name = name
	return r
}

// WithContentType sets the content type.
func (r Ref) WithContentType(contentType string) Ref {
	r.ContentType = contentType
	return r
}

// WithSize sets the artifact size.
func (r Ref) WithSize(size int64) Ref {
	r.Size = size
	return r
}

// WithChecksum sets the checksum.
func (r Ref) WithChecksum(checksum string) Ref {
	r.Checksum = checksum
	return r
}

// WithMetadata adds metadata to the artifact.
func (r Ref) WithMetadata(key, value string) Ref {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
	return r
}

// IsValid returns true if the reference has a valid ID.
func (r Ref) IsValid() bool {
	return r.ID != ""
}

// String returns a string representation of the reference.
func (r Ref) String() string {
	if r.Name != "" {
		return r.Name + " (" + r.ID + ")"
	}
	return r.ID
}
