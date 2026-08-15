package sandbox

import (
	"path/filepath"
	"strings"
)

// CommandAllowed reports whether argv0 matches the allowlist.
// Entries may be basenames ("git") or absolute paths ("/usr/bin/git").
// Matching is exact on the entry: basename entries match argv0's basename
// or argv0 itself; absolute entries match argv0 exactly.
// An empty allowlist denies all commands.
func CommandAllowed(argv0 string, allowlist []string) bool {
	if argv0 == "" || len(allowlist) == 0 {
		return false
	}
	base := filepath.Base(argv0)
	for _, a := range allowlist {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if a == argv0 || a == base {
			return true
		}
	}
	return false
}
