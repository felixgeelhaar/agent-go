package sandbox_test

import (
	"path/filepath"
	"testing"

	"go.klarlabs.de/agent/sandbox"
)

// Pack contract: every BaseDir-jailed pack must reject these inputs via SafePath.
func TestContract_PathJail(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	rejects := []string{
		"",
		"/etc/passwd",
		"../escape",
		filepath.Join("..", "..", "etc", "passwd"),
		filepath.Join("ok", "..", "..", "etc"),
	}
	for _, p := range rejects {
		if _, err := sandbox.SafePath(base, p); err == nil {
			t.Errorf("SafePath(%q) should reject", p)
		}
	}

	ok, err := sandbox.SafePath(base, filepath.Join("a", "b.txt"))
	if err != nil {
		t.Fatalf("SafePath allowed path: %v", err)
	}
	if !sandbox.IsUnderBase(base, ok) {
		t.Fatalf("resolved path %q not under base", ok)
	}
}

// Pack contract: host allowlists deny-all when empty.
func TestContract_HostAllowlist(t *testing.T) {
	t.Parallel()
	if sandbox.HostAllowed("example.com", nil) {
		t.Fatal("empty allowlist must deny")
	}
	if sandbox.HostAllowed("example.com", []string{}) {
		t.Fatal("empty allowlist must deny")
	}
	if !sandbox.HostAllowed("metrics.prod.example.com", []string{"*.example.com"}) {
		t.Fatal("wildcard should allow")
	}
}

// Pack contract: command allowlists deny-all when empty; argv-only model.
func TestContract_CommandAllowlist(t *testing.T) {
	t.Parallel()
	if sandbox.CommandAllowed("echo", nil) {
		t.Fatal("empty allowlist must deny")
	}
	if sandbox.CommandAllowed("rm", []string{"echo"}) {
		t.Fatal("unlisted command must deny")
	}
	if !sandbox.CommandAllowed("echo", []string{"echo"}) {
		t.Fatal("listed basename must allow")
	}
}
