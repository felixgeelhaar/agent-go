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

// MaxIDLength bounds the length of an artifact ID. Filesystem-backed stores use
// the ID as a single path component, and 255 bytes is the common per-component
// limit on Linux and macOS.
const MaxIDLength = 255

// IsValidID reports whether id is a well-formed artifact ID.
//
// A valid ID is non-empty, at most MaxIDLength bytes long, is neither "." nor
// "..", and consists only of ASCII letters, digits, '-', '_' and '.'.
//
// The charset is deliberately narrow. Stores use the ID verbatim as a path
// component (filesystem) or object key (GCS, S3), so path separators, parent
// directory references and control bytes must never appear in one. A Ref is a
// plain JSON value that can round-trip through tool output and model-authored
// input, so an ID arriving at Retrieve/Delete/Exists/Metadata is not necessarily
// an ID this process generated — validation is the store's contract with its
// caller, not an assumption it may make.
//
// The format accepts every ID the bundled stores emit: the filesystem store's
// "<unix-nanos>-<random>" and the GCS store's UUIDs.
func IsValidID(id string) bool {
	if id == "" || len(id) > MaxIDLength {
		return false
	}
	if id == "." || id == ".." {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// IsValid returns true if the reference carries a well-formed ID.
//
// See IsValidID for the accepted format.
func (r Ref) IsValid() bool {
	return IsValidID(r.ID)
}

// String returns a string representation of the reference.
func (r Ref) String() string {
	if r.Name != "" {
		return r.Name + " (" + r.ID + ")"
	}
	return r.ID
}
