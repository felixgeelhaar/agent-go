package secrets_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	secrets "go.klarlabs.de/agent/contrib/pack-secrets"
	"go.klarlabs.de/agent/domain/tool"
)

func toolMap(store secrets.SecretStore) map[string]tool.Tool {
	p := secrets.Pack(store)
	m := make(map[string]tool.Tool, len(p.Tools))
	for _, tt := range p.Tools {
		m[tt.Name()] = tt
	}
	return m
}

func TestPackNotStub(t *testing.T) {
	p := secrets.Pack(secrets.NewMemoryStore())
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
	if len(p.Tools) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(p.Tools))
	}
}

func TestMemoryStoreLifecycle(t *testing.T) {
	store := secrets.NewMemoryStore()
	tools := toolMap(store)
	ctx := context.Background()

	_, err := tools["secrets_set"].Execute(ctx, json.RawMessage(`{"key":"db/pass","value":"s3cret"}`))
	if err != nil {
		t.Fatal(err)
	}

	res, err := tools["secrets_get"].Execute(ctx, json.RawMessage(`{"key":"db/pass"}`))
	if err != nil {
		t.Fatal(err)
	}
	var got secrets.Secret
	if err := json.Unmarshal(res.Output, &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "s3cret" || got.Version != 1 {
		t.Fatalf("unexpected secret: %+v", got)
	}

	res, err = tools["secrets_list"].Execute(ctx, json.RawMessage(`{"prefix":"db/"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(res.Output) {
		t.Fatal("invalid list json")
	}
	var list struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(res.Output, &list)
	if list.Count != 1 {
		t.Fatalf("expected 1, got %d", list.Count)
	}
	if strings.Contains(string(res.Output), "s3cret") {
		t.Fatal("list must not include secret values")
	}

	_, err = tools["secrets_rotate"].Execute(ctx, json.RawMessage(`{"key":"db/pass"}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err = tools["secrets_version"].Execute(ctx, json.RawMessage(`{"key":"db/pass","version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(res.Output, &got)
	if got.Version != 1 || got.Value != "s3cret" {
		t.Fatalf("version 1 should be original: %+v", got)
	}

	_, err = tools["secrets_delete"].Execute(ctx, json.RawMessage(`{"key":"db/pass"}`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	store, err := secrets.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Set(ctx, "a", "1"); err != nil {
		t.Fatal(err)
	}
	// reopen
	store2, err := secrets.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	sec, err := store2.Get(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if sec.Value != "1" {
		t.Fatalf("got %q", sec.Value)
	}
}
