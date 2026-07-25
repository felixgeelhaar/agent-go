// Package filesystem provides filesystem-based storage implementations.
package filesystem

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/artifact"
)

// ArtifactStore implements artifact.Store using the local filesystem.
type ArtifactStore struct {
	basePath string
}

// NewArtifactStore creates a new filesystem artifact store.
func NewArtifactStore(basePath string) (*ArtifactStore, error) {
	// Ensure base path exists with restrictive permissions (G301 fix)
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	return &ArtifactStore{basePath: basePath}, nil
}

// Store saves content and returns a stable reference.
func (s *ArtifactStore) Store(ctx context.Context, content io.Reader, opts artifact.StoreOptions) (artifact.Ref, error) {
	// Generate unique ID
	id := generateArtifactID()

	// Create artifact directory with restrictive permissions (G301 fix)
	artifactPath, err := s.artifactPath(id)
	if err != nil {
		// Unreachable: generateArtifactID only emits IDs that pass validation.
		// Fail closed rather than trust that invariant.
		return artifact.Ref{}, err
	}
	if err := os.MkdirAll(artifactPath, 0750); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to create artifact path: %w", err)
	}

	// Write content to file
	contentPath := filepath.Join(artifactPath, "content")
	// #nosec G304 -- path is constructed from internally generated artifact ID, not user input
	file, err := os.Create(contentPath)
	if err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to create content file: %w", err)
	}

	// Compute checksum while writing
	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	size, err := io.Copy(writer, content)
	if err != nil {
		file.Close()               // #nosec G104 -- best-effort cleanup in error path
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to write content: %w", err)
	}

	// Close content file and check for write errors (G104 fix)
	if err := file.Close(); err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to close content file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

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

	// Write metadata file
	metaPath := filepath.Join(artifactPath, "metadata.json")
	// #nosec G304 -- path is constructed from internally generated artifact ID, not user input
	metaFile, err := os.Create(metaPath)
	if err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to create metadata file: %w", err)
	}

	if err := json.NewEncoder(metaFile).Encode(ref); err != nil {
		metaFile.Close()           // #nosec G104 -- best-effort cleanup in error path
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Close metadata file and check for write errors (G104 fix)
	if err := metaFile.Close(); err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to close metadata file: %w", err)
	}

	return ref, nil
}

// Retrieve retrieves the content for an artifact reference.
func (s *ArtifactStore) Retrieve(_ context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	if !ref.IsValid() {
		return nil, artifact.ErrInvalidRef
	}

	dir, err := s.artifactPath(ref.ID)
	if err != nil {
		return nil, err
	}

	contentPath := filepath.Join(dir, "content")
	// #nosec G304 -- ref.ID is caller-supplied; artifactPath validates its format
	// and confines the result to basePath before it reaches os.Open.
	file, err := os.Open(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, artifact.ErrArtifactNotFound
		}
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}

	return file, nil
}

// Delete removes an artifact.
func (s *ArtifactStore) Delete(_ context.Context, ref artifact.Ref) error {
	if !ref.IsValid() {
		return artifact.ErrInvalidRef
	}

	artifactPath, err := s.artifactPath(ref.ID)
	if err != nil {
		return err
	}

	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		return artifact.ErrArtifactNotFound
	}

	if err := os.RemoveAll(artifactPath); err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists.
func (s *ArtifactStore) Exists(_ context.Context, ref artifact.Ref) (bool, error) {
	if !ref.IsValid() {
		return false, artifact.ErrInvalidRef
	}

	dir, err := s.artifactPath(ref.ID)
	if err != nil {
		return false, err
	}

	contentPath := filepath.Join(dir, "content")
	_, err = os.Stat(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Metadata retrieves the metadata for an artifact without content.
func (s *ArtifactStore) Metadata(_ context.Context, ref artifact.Ref) (artifact.Ref, error) {
	if !ref.IsValid() {
		return artifact.Ref{}, artifact.ErrInvalidRef
	}

	dir, err := s.artifactPath(ref.ID)
	if err != nil {
		return artifact.Ref{}, err
	}

	metaPath := filepath.Join(dir, "metadata.json")
	// #nosec G304 -- ref.ID is caller-supplied; artifactPath validates its format
	// and confines the result to basePath before it reaches os.Open.
	file, err := os.Open(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return artifact.Ref{}, artifact.ErrArtifactNotFound
		}
		return artifact.Ref{}, fmt.Errorf("failed to open metadata: %w", err)
	}
	defer file.Close() // #nosec G104 -- read-only operation, close error is non-critical

	var storedRef artifact.Ref
	if err := json.NewDecoder(file).Decode(&storedRef); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return storedRef, nil
}

// artifactPath returns the directory path for an artifact, confined to basePath.
//
// filepath.Join cleans "../" segments but does not confine: joining basePath with
// an ID of "../../etc" yields a path outside the store, which Delete would then
// hand to os.RemoveAll. Artifact refs are caller-supplied on every read and delete
// path, so the ID is validated and the result is checked before any syscall.
//
// Two independent checks run here:
//
//  1. The ID must satisfy artifact.IsValidID — no separators, no "..", ASCII only.
//  2. The cleaned join must remain a strict descendant of filepath.Clean(basePath).
//
// Check 2 is unreachable while check 1 holds. It is kept deliberately, so that
// relaxing the ID format later cannot silently reintroduce a traversal.
//
// Neither check follows symlinks: a symlink planted *inside* basePath by a
// separate process with write access there could still redirect a read. That
// requires an attacker who already controls the store directory, which is
// outside this store's threat model.
func (s *ArtifactStore) artifactPath(id string) (string, error) {
	if !artifact.IsValidID(id) {
		return "", artifact.ErrInvalidRef
	}
	return confineToBase(s.basePath, id)
}

// confineToBase joins id under base and rejects any result that is not a strict
// descendant of base. It is kept separate from ID validation so that each layer
// can be exercised on its own.
func confineToBase(base, id string) (string, error) {
	cleanBase := filepath.Clean(base)
	joined := filepath.Clean(filepath.Join(cleanBase, id))
	if !strings.HasPrefix(joined, cleanBase+string(filepath.Separator)) {
		return "", artifact.ErrInvalidRef
	}
	return joined, nil
}

// idRandomLength is the number of random characters appended to an artifact ID.
// 16 characters over a 36-symbol alphabet is roughly 82 bits of entropy.
const idRandomLength = 16

// generateArtifactID creates a unique, unpredictable artifact ID.
//
// The Unix-nanosecond prefix keeps IDs roughly sortable by creation time; the
// suffix — not the prefix — is what makes an ID unguessable.
func generateArtifactID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(idRandomLength))
}

// randomString generates a cryptographically random alphanumeric string of
// length n, drawn uniformly from [a-z0-9].
//
// The alphabet has 36 symbols, which does not divide 256, so bytes are rejection
// sampled rather than reduced modulo 36: taking the remainder directly would make
// the first four symbols ~14% more likely than the rest.
//
// This function does not return an error. crypto/rand.Read is documented never to
// fail — since Go 1.24 the runtime treats an unavailable system CSPRNG as fatal —
// so the only way to reach the panic below would be a broken rand.Reader override.
// Panicking is deliberate: an artifact ID that silently fell back to a weak source
// would be predictable, which is the bug this replaced.
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	// Largest multiple of len(charset) that fits in a byte; higher values are
	// discarded to keep the distribution uniform.
	const limit = 256 - (256 % len(charset)) // 252

	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			panic("filesystem: crypto/rand unavailable: " + err.Error())
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue // biased sample, draw again
			}
			out = append(out, charset[int(b)%len(charset)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out)
}
