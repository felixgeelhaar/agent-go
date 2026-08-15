package vectordb_test

import (
	"context"
	"encoding/json"
	"testing"

	vectordb "go.klarlabs.de/agent/contrib/pack-vectordb"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestPack(t *testing.T) {
	store := memory.NewKnowledgeStore(3)
	p := vectordb.Pack(store)
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Metadata["status"] == "stub" {
		t.Fatal("pack should not be marked stub")
	}
	if len(p.Tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(p.Tools))
	}
}

func TestUpsertQueryFetchDelete(t *testing.T) {
	store := memory.NewKnowledgeStore(3)
	p := vectordb.Pack(store)
	tools := map[string]tool.Tool{}
	for _, tt := range p.Tools {
		tools[tt.Name()] = tt
	}

	ctx := context.Background()
	_, err := tools["vector_upsert"].Execute(ctx, json.RawMessage(`{
		"id":"a","embedding":[1,0,0],"text":"alpha","metadata":{"k":"v"}
	}`))
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	res, err := tools["vector_query"].Execute(ctx, json.RawMessage(`{"embedding":[1,0,0],"top_k":1}`))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var qout struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(res.Output, &qout); err != nil {
		t.Fatal(err)
	}
	if qout.Count != 1 {
		t.Fatalf("expected 1 result, got %d", qout.Count)
	}

	res, err = tools["vector_fetch"].Execute(ctx, json.RawMessage(`{"id":"a"}`))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	var fetched struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(res.Output, &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.ID != "a" || fetched.Text != "alpha" {
		t.Fatalf("unexpected fetch: %+v", fetched)
	}

	res, err = tools["vector_describe_index"].Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("describe: %v", err)
	}

	_, err = tools["vector_delete"].Execute(ctx, json.RawMessage(`{"id":"a"}`))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeleteIndexRequiresConfirm(t *testing.T) {
	store := memory.NewKnowledgeStore(2)
	p := vectordb.Pack(store)
	var del tool.Tool
	for _, tt := range p.Tools {
		if tt.Name() == "vector_delete_index" {
			del = tt
		}
	}
	_, err := del.Execute(context.Background(), json.RawMessage(`{"confirm":false}`))
	if err == nil {
		t.Fatal("expected error without confirm")
	}
}
