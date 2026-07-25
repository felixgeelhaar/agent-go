package filesystem

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/artifact"
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

func TestNewArtifactStore_InvalidPath(t *testing.T) {
	t.Parallel()

	// Use a path under a file (not a directory) to force MkdirAll to fail.
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "afile")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(filePath, "subdir")

	_, err := NewArtifactStore(invalidPath)
	if err == nil {
		t.Fatal("expected error when base path cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create artifact directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestArtifactStore_Store_MkdirAllError(t *testing.T) {
	t.Parallel()

	// Create store, then make basePath read-only so artifact subdirectory creation fails.
	tempDir := t.TempDir()
	store, err := NewArtifactStore(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove write permission on the basePath.
	if err := os.Chmod(tempDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(tempDir, 0750) })

	ctx := context.Background()
	_, err = store.Store(ctx, strings.NewReader("data"), artifact.DefaultStoreOptions())
	if err == nil {
		t.Fatal("expected error when artifact directory cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create artifact path") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestArtifactStore_Store_CopyError(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, err := NewArtifactStore(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// Use a reader that returns an error during read.
	_, err = store.Store(ctx, &errorReader{}, artifact.DefaultStoreOptions())
	if err == nil {
		t.Fatal("expected error when content read fails")
	}
	if !strings.Contains(err.Error(), "failed to write content") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestArtifactStore_Store_EmptyContent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, _ := NewArtifactStore(tempDir)
	ctx := context.Background()

	ref, err := store.Store(ctx, strings.NewReader(""), artifact.DefaultStoreOptions())
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if ref.Size != 0 {
		t.Errorf("Size = %d, want 0", ref.Size)
	}
}

func TestArtifactStore_Store_LargeContent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, _ := NewArtifactStore(tempDir)
	ctx := context.Background()

	// 1MB of data
	data := bytes.Repeat([]byte("A"), 1<<20)
	ref, err := store.Store(ctx, bytes.NewReader(data), artifact.DefaultStoreOptions().WithName("large.bin"))
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if ref.Size != int64(len(data)) {
		t.Errorf("Size = %d, want %d", ref.Size, len(data))
	}

	// Verify roundtrip
	reader, err := store.Retrieve(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, _ := io.ReadAll(reader)
	if !bytes.Equal(got, data) {
		t.Error("retrieved content does not match stored content")
	}
}

func TestArtifactStore_Store_NoName(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, _ := NewArtifactStore(tempDir)
	ctx := context.Background()

	opts := artifact.StoreOptions{
		ContentType:     "text/plain",
		ComputeChecksum: true,
	}
	ref, err := store.Store(ctx, strings.NewReader("content"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name != "" {
		t.Errorf("expected empty name, got %q", ref.Name)
	}
}

func TestArtifactStore_Metadata_CorruptedJSON(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, _ := NewArtifactStore(tempDir)
	ctx := context.Background()

	// Store a valid artifact first.
	ref, err := store.Store(ctx, strings.NewReader("content"), artifact.DefaultStoreOptions())
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the metadata file.
	metaPath := filepath.Join(tempDir, ref.ID, "metadata.json")
	if err := os.WriteFile(metaPath, []byte("not json{{{"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Metadata(ctx, ref)
	if err == nil {
		t.Fatal("expected error for corrupted metadata")
	}
	if !strings.Contains(err.Error(), "failed to decode metadata") {
		t.Errorf("unexpected error: %v", err)
	}
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read error")
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

// TestArtifactStore_RejectsPathTraversal is a regression test for a path
// traversal in the four methods that take a caller-supplied ref. They gated on
// Ref.IsValid(), which only checked for a non-empty ID, and then handed the ID
// to filepath.Join — which cleans "../" but does not confine. Before the fix,
// Retrieve/Exists/Metadata read files outside basePath and Delete handed the
// escaped path to os.RemoveAll.
func TestArtifactStore_RejectsPathTraversal(t *testing.T) {
	t.Parallel()

	// root/
	//   base/     <- the artifact store
	//   outside/  <- must never be read or removed
	root := t.TempDir()
	base := filepath.Join(root, "base")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0750); err != nil {
		t.Fatal(err)
	}
	// Shaped like a real artifact directory so that, without the fix, every one
	// of the four methods succeeds rather than merely erroring differently.
	victims := map[string]string{
		"secret.txt":    "top secret",
		"content":       "top secret",
		"metadata.json": `{"id":"escaped"}`,
	}
	for name, data := range victims {
		if err := os.WriteFile(filepath.Join(outside, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}

	store, err := NewArtifactStore(base)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	hostileIDs := []string{
		"../outside",
		"../../base/../outside",
		"..",
		"../",
		"./../outside",
		outside, // absolute path
		"sub/dir",
		"nul\x00byte",
		strings.Repeat("a", 4096),
	}

	for _, id := range hostileIDs {
		ref := artifact.NewRef(id)

		if _, err := store.Retrieve(ctx, ref); err != artifact.ErrInvalidRef {
			t.Errorf("Retrieve(%q) error = %v, want ErrInvalidRef", id, err)
		}
		if _, err := store.Exists(ctx, ref); err != artifact.ErrInvalidRef {
			t.Errorf("Exists(%q) error = %v, want ErrInvalidRef", id, err)
		}
		if _, err := store.Metadata(ctx, ref); err != artifact.ErrInvalidRef {
			t.Errorf("Metadata(%q) error = %v, want ErrInvalidRef", id, err)
		}
		if err := store.Delete(ctx, ref); err != artifact.ErrInvalidRef {
			t.Errorf("Delete(%q) error = %v, want ErrInvalidRef", id, err)
		}
	}

	// Nothing outside the store may have been touched.
	for name := range victims {
		if _, err := os.Stat(filepath.Join(outside, name)); err != nil {
			t.Errorf("file outside basePath was affected: %v", err)
		}
	}
}

// TestConfineToBase exercises the confinement layer on its own, with IDs that
// artifact.IsValidID already rejects. The two checks are deliberately redundant;
// this test fails if the confinement is dropped in favour of validation alone.
func TestConfineToBase(t *testing.T) {
	t.Parallel()

	base := "/srv/artifacts"

	escapes := []string{"..", "../outside", "sub/../../outside", "../artifacts-evil", "."}
	for _, id := range escapes {
		if got, err := confineToBase(base, id); err != artifact.ErrInvalidRef {
			t.Errorf("confineToBase(%q, %q) = %q, %v; want ErrInvalidRef", base, id, got, err)
		}
	}

	id := "1700000000000000000-abcdefgh"
	got, err := confineToBase(base, id)
	if err != nil {
		t.Fatalf("confineToBase(%q) error = %v", id, err)
	}
	if want := filepath.Join(base, id); got != want {
		t.Errorf("confineToBase(%q) = %q, want %q", id, got, want)
	}
}

// TestRandomString_NotDerivableFromClock is a regression test for an ID
// generator that filled every character from time.Now().UnixNano()%36 with a
// 1ns sleep between characters. Because the clock advances by a near-constant
// amount per iteration, the output collapsed onto a handful of symbols: a
// measured 2000 samples yielded only 649 distinct strings drawn from 9 of the
// 36 alphabet characters. With a CSPRNG both counts are pinned by probability,
// not by scheduling.
func TestRandomString_NotDerivableFromClock(t *testing.T) {
	t.Parallel()

	const (
		samples = 2000
		length  = 8
		charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	)

	seen := make(map[string]struct{}, samples)
	chars := make(map[byte]struct{}, len(charset))
	for i := 0; i < samples; i++ {
		s := randomString(length)
		if len(s) != length {
			t.Fatalf("randomString(%d) length = %d", length, len(s))
		}
		seen[s] = struct{}{}
		for j := 0; j < len(s); j++ {
			if !strings.ContainsRune(charset, rune(s[j])) {
				t.Fatalf("randomString() produced invalid character %q", s[j])
			}
			chars[s[j]] = struct{}{}
		}
	}

	// 2000 draws from 36^8 (~2.8e12) values: the chance of even one collision is
	// under one in a million, so any repeat means the source is not random.
	if len(seen) != samples {
		t.Errorf("randomString() produced %d distinct values out of %d samples; "+
			"output is not cryptographically random", len(seen), samples)
	}

	// 16000 characters over a 36-symbol alphabet: the chance of missing any one
	// symbol is ~36*(35/36)^16000, i.e. astronomically small.
	if len(chars) != len(charset) {
		t.Errorf("randomString() only ever emitted %d of %d alphabet characters; "+
			"output is clock-derived, not random", len(chars), len(charset))
	}
}

// TestGenerateArtifactID_UnpredictableSuffix asserts the property that matters
// for path safety: knowing when an artifact was written must not be enough to
// reconstruct its ID. The timestamp prefix is guessable by design, so the check
// is on the random suffix alone.
func TestGenerateArtifactID_UnpredictableSuffix(t *testing.T) {
	t.Parallel()

	const samples = 1000

	suffixes := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		id := generateArtifactID()
		if !artifact.IsValidID(id) {
			t.Fatalf("generateArtifactID() = %q, which artifact.IsValidID rejects", id)
		}
		idx := strings.LastIndex(id, "-")
		if idx < 0 {
			t.Fatalf("generateArtifactID() = %q, want a <nanos>-<random> shape", id)
		}
		suffix := id[idx+1:]
		if len(suffix) != idRandomLength {
			t.Fatalf("suffix %q length = %d, want %d", suffix, len(suffix), idRandomLength)
		}
		suffixes[suffix] = struct{}{}
	}

	if len(suffixes) != samples {
		t.Errorf("generateArtifactID() produced %d distinct suffixes out of %d; "+
			"IDs are predictable from the clock", len(suffixes), samples)
	}
}
