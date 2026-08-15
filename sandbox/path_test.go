package sandbox_test

import (
	"path/filepath"
	"testing"

	"go.klarlabs.de/agent/sandbox"
)

func TestSafePath_RejectsEscape(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	if _, err := sandbox.SafePath(base, "../etc/passwd"); err == nil {
		t.Fatal("expected escape rejection")
	}
	if _, err := sandbox.SafePath(base, "/etc/passwd"); err == nil {
		t.Fatal("expected absolute rejection")
	}
	if _, err := sandbox.SafePath(base, ""); err == nil {
		t.Fatal("expected empty rejection")
	}

	got, err := sandbox.SafePath(base, "ok/file.txt")
	if err != nil {
		t.Fatalf("SafePath: %v", err)
	}
	want := filepath.Join(base, "ok", "file.txt")
	if got != want {
		t.Fatalf("SafePath = %q, want %q", got, want)
	}
}

func TestHostAllowed(t *testing.T) {
	t.Parallel()
	if sandbox.HostAllowed("evil.com", nil) {
		t.Fatal("empty allowlist must deny")
	}
	allowed := []string{"api.example.com", "*.trusted.io"}
	if !sandbox.HostAllowed("api.example.com", allowed) {
		t.Fatal("exact host should allow")
	}
	if !sandbox.HostAllowed("api.example.com:443", allowed) {
		t.Fatal("host:port should allow")
	}
	if !sandbox.HostAllowed("a.trusted.io", allowed) {
		t.Fatal("wildcard should allow")
	}
	if sandbox.HostAllowed("evil.com", allowed) {
		t.Fatal("unlisted host must deny")
	}
}

func TestCommandAllowed(t *testing.T) {
	t.Parallel()
	if sandbox.CommandAllowed("echo", nil) {
		t.Fatal("empty allowlist must deny")
	}
	allow := []string{"echo", "/usr/bin/true"}
	if !sandbox.CommandAllowed("echo", allow) {
		t.Fatal("basename should allow")
	}
	if !sandbox.CommandAllowed("/bin/echo", allow) {
		t.Fatal("path with allowlisted basename should allow")
	}
	if !sandbox.CommandAllowed("/usr/bin/true", allow) {
		t.Fatal("absolute allowlist entry should allow")
	}
	if sandbox.CommandAllowed("rm", allow) {
		t.Fatal("unlisted command must deny")
	}
}
