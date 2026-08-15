package llm_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	llm "go.klarlabs.de/agent/contrib/pack-llm"
	"go.klarlabs.de/agent/domain/tool"
)

type mockCompleter struct{}

func (mockCompleter) Name() string { return "mock" }

func (mockCompleter) Complete(_ context.Context, messages []llm.Message, _ llm.CompleteOptions) (llm.CompleteResult, error) {
	var last string
	for _, m := range messages {
		if m.Role == "user" {
			last = m.Content
		}
	}
	return llm.CompleteResult{Content: "OUT:" + last, Model: "mock-1", Usage: llm.Usage{TotalTokens: 3}}, nil
}

type mockEmbedder struct{}

func (mockEmbedder) EmbedText(_ context.Context, text string, _ string) ([]float64, error) {
	return []float64{float64(len(text)), 1, 0}, nil
}

func getTools(t *testing.T) map[string]tool.Tool {
	t.Helper()
	p := llm.Pack(llm.Config{Completer: mockCompleter{}, Embeddings: mockEmbedder{}})
	m := make(map[string]tool.Tool, len(p.Tools))
	for _, tt := range p.Tools {
		m[tt.Name()] = tt
	}
	return m
}

func TestPack(t *testing.T) {
	p := llm.Pack(llm.Config{Completer: mockCompleter{}})
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
	if len(p.Tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(p.Tools))
	}
}

func TestCompleteAndChat(t *testing.T) {
	tools := getTools(t)
	ctx := context.Background()

	res, err := tools["llm_complete"].Execute(ctx, json.RawMessage(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Output), "OUT:hello") {
		t.Fatalf("unexpected output: %s", res.Output)
	}

	res, err = tools["llm_chat"].Execute(ctx, json.RawMessage(`{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Output), "OUT:hi") {
		t.Fatalf("unexpected chat output: %s", res.Output)
	}
}

func TestSummarizeClassifyTranslateEmbed(t *testing.T) {
	tools := getTools(t)
	ctx := context.Background()

	if _, err := tools["llm_summarize"].Execute(ctx, json.RawMessage(`{"text":"long text"}`)); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if _, err := tools["llm_classify"].Execute(ctx, json.RawMessage(`{"text":"x","categories":["a","b"]}`)); err != nil {
		t.Fatalf("classify: %v", err)
	}
	if _, err := tools["llm_translate"].Execute(ctx, json.RawMessage(`{"text":"hello","target_lang":"de"}`)); err != nil {
		t.Fatalf("translate: %v", err)
	}
	res, err := tools["llm_embed"].Execute(ctx, json.RawMessage(`{"text":"abc"}`))
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	var eout struct {
		Dimension int `json:"dimension"`
	}
	_ = json.Unmarshal(res.Output, &eout)
	if eout.Dimension != 3 {
		t.Fatalf("expected dim 3, got %d", eout.Dimension)
	}
}

func TestEmbedRequiresProvider(t *testing.T) {
	p := llm.Pack(llm.Config{Completer: mockCompleter{}})
	var embed tool.Tool
	for _, tt := range p.Tools {
		if tt.Name() == "llm_embed" {
			embed = tt
		}
	}
	_, err := embed.Execute(context.Background(), json.RawMessage(`{"text":"x"}`))
	if err == nil {
		t.Fatal("expected error without embedding provider")
	}
}
