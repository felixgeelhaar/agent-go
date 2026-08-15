package shell_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	shell "go.klarlabs.de/agent/contrib/pack-shell"
	"go.klarlabs.de/agent/domain/tool"
)

func toolMap(cfg shell.Config) map[string]tool.Tool {
	p := shell.Pack(cfg)
	m := make(map[string]tool.Tool, len(p.Tools))
	for _, tt := range p.Tools {
		m[tt.Name()] = tt
	}
	return m
}

func TestPackNotStub(t *testing.T) {
	p := shell.Pack(shell.Config{Allowlist: []string{"echo"}, BaseDir: t.TempDir()})
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
}

func TestShellExecAllowlistAndCapture(t *testing.T) {
	dir := t.TempDir()
	tools := toolMap(shell.Config{Allowlist: []string{"echo"}, BaseDir: dir})
	ctx := context.Background()

	res, err := tools["shell_exec"].Execute(ctx, json.RawMessage(`{"argv":["echo","hello"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != 0 || out.Stdout != "hello\n" {
		if runtime.GOOS == "windows" {
			t.Skip("windows echo differs")
		}
		t.Fatalf("unexpected: %+v", out)
	}

	_, err = tools["shell_exec"].Execute(ctx, json.RawMessage(`{"argv":["rm","-rf","/"]}`))
	if err == nil {
		t.Fatal("expected allowlist rejection")
	}
}

func TestCwdJail(t *testing.T) {
	dir := t.TempDir()
	tools := toolMap(shell.Config{Allowlist: []string{"echo"}, BaseDir: dir})
	_, err := tools["shell_exec"].Execute(context.Background(), json.RawMessage(
		`{"argv":["echo","x"],"cwd":".."}`))
	if err == nil {
		t.Fatal("expected cwd jail rejection")
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = tools["shell_exec"].Execute(context.Background(), json.RawMessage(
		`{"argv":["echo","x"],"cwd":"sub"}`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestShellScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script test is unix")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found")
	}
	dir := t.TempDir()
	tools := toolMap(shell.Config{Allowlist: []string{"sh"}, BaseDir: dir})
	res, err := tools["shell_script"].Execute(context.Background(), json.RawMessage(
		`{"interpreter":"sh","script":"echo scripted"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Stdout string `json:"stdout"`
	}
	_ = json.Unmarshal(res.Output, &out)
	if out.Stdout != "scripted\n" {
		t.Fatalf("got %q", out.Stdout)
	}
}

func TestBackgroundJob(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	tools := toolMap(shell.Config{Allowlist: []string{"echo"}, BaseDir: dir})
	res, err := tools["shell_exec_background"].Execute(context.Background(), json.RawMessage(
		`{"argv":["echo","bg"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		JobID string `json:"job_id"`
		PID   int    `json:"pid"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.JobID == "" || out.PID == 0 {
		t.Fatalf("unexpected: %+v", out)
	}
}
