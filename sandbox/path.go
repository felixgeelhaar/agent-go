// Package sandbox provides shared jail helpers for contrib tool packs.
//
// Trust properties enforced here:
//   - Paths: reject absolute inputs; resolve under BaseDir; refuse ".." escapes
//   - Hosts: exact or "*.suffix" allowlists (deny-all when empty)
//   - Commands: exact basename or absolute-path allowlists (deny-all when empty)
package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafePath resolves userPath relative to baseDir and ensures the result stays
// inside the sandbox. Absolute user paths are rejected (they discard baseDir
// under filepath.Join).
func SafePath(baseDir, userPath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("path traversal attempt: empty path")
	}
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("path traversal attempt: %s", userPath)
	}
	fullPath := filepath.Join(baseDir, filepath.Clean(userPath))
	if !IsUnderBase(baseDir, fullPath) {
		return "", fmt.Errorf("path traversal attempt: %s", userPath)
	}
	return fullPath, nil
}

// IsUnderBase reports whether path is inside base (after Abs).
func IsUnderBase(base, path string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}
