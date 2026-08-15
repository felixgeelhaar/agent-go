package search_test

import (
	"context"
	"encoding/json"
	"testing"

	search "go.klarlabs.de/agent/contrib/pack-search"
	"go.klarlabs.de/agent/domain/tool"
)

func toolMap(e search.SearchEngine) map[string]tool.Tool {
	p := search.Pack(e)
	m := make(map[string]tool.Tool, len(p.Tools))
	for _, tt := range p.Tools {
		m[tt.Name()] = tt
	}
	return m
}

func TestPackNotStub(t *testing.T) {
	p := search.Pack(search.NewMemoryEngine())
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
	if len(p.Tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(p.Tools))
	}
}

func TestIndexQuerySuggestAggregate(t *testing.T) {
	eng := search.NewMemoryEngine()
	tools := toolMap(eng)
	ctx := context.Background()

	_, err := tools["search_index"].Execute(ctx, json.RawMessage(
		`{"index":"docs","id":"1","body":"golang search engine","fields":{"lang":"go"}}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = tools["search_bulk_index"].Execute(ctx, json.RawMessage(
		`{"index":"docs","docs":[{"id":"2","body":"rust search tools","fields":{"lang":"rust"}},{"id":"3","body":"golang tools","fields":{"lang":"go"}}]}`))
	if err != nil {
		t.Fatal(err)
	}

	res, err := tools["search_query"].Execute(ctx, json.RawMessage(
		`{"index":"docs","query":"golang","limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	var qout struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(res.Output, &qout)
	if qout.Count < 1 {
		t.Fatalf("expected hits, got %d", qout.Count)
	}

	res, err = tools["search_suggest"].Execute(ctx, json.RawMessage(
		`{"index":"docs","prefix":"gol"}`))
	if err != nil {
		t.Fatal(err)
	}
	var sout struct {
		Suggestions []string `json:"suggestions"`
	}
	_ = json.Unmarshal(res.Output, &sout)
	if len(sout.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}

	res, err = tools["search_aggregate"].Execute(ctx, json.RawMessage(
		`{"index":"docs","field":"lang"}`))
	if err != nil {
		t.Fatal(err)
	}
	var aout struct {
		Buckets []search.AggregateBucket `json:"buckets"`
	}
	_ = json.Unmarshal(res.Output, &aout)
	if len(aout.Buckets) < 1 {
		t.Fatal("expected aggregate buckets")
	}

	_, err = tools["search_delete"].Execute(ctx, json.RawMessage(`{"index":"docs","id":"1"}`))
	if err != nil {
		t.Fatal(err)
	}

	res, err = tools["search_indices"].Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var iout struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(res.Output, &iout)
	if iout.Count != 1 {
		t.Fatalf("expected 1 index, got %d", iout.Count)
	}
}
