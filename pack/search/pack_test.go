package search_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/pack/search"
)

func TestNew(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}

	tools := pack.Tools
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	// Verify expected tools (read-only mode)
	toolNames := make(map[string]bool)
	for _, tl := range tools {
		toolNames[tl.Name()] = true
	}

	expectedTools := []string{"search_list_indices", "search_query", "search_get_document"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}

	// Write tools should not be present
	if toolNames["search_index_document"] {
		t.Error("search_index_document should not be present in read-only mode")
	}
	if toolNames["search_delete_document"] {
		t.Error("search_delete_document should not be present in read-only mode")
	}
}

func TestNew_WithWriteAccess(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["search_index_document"] {
		t.Error("search_index_document should be present with write access")
	}
	// Delete should not be present without explicit delete access
	if toolNames["search_delete_document"] {
		t.Error("search_delete_document should not be present without delete access")
	}
}

func TestNew_WithDeleteAccess(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithWriteAccess(), search.WithDeleteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["search_delete_document"] {
		t.Error("search_delete_document should be present with delete access")
	}
}

func TestNew_NilProvider(t *testing.T) {
	_, err := search.New(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestListIndices(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	// Create some indices
	provider.CreateIndex("index-1")
	provider.CreateIndex("index-2")
	provider.CreateIndex("index-3")

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find search_list_indices tool
	var listIndicesTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_list_indices" {
			listIndicesTool = tl
			break
		}
	}

	if listIndicesTool == nil {
		t.Fatal("search_list_indices tool not found")
	}

	// Execute tool
	result, err := listIndicesTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("search_list_indices failed: %v", err)
	}

	var output struct {
		Provider string            `json:"provider"`
		Indices  []search.IndexInfo `json:"indices"`
		Count    int               `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Provider != "memory" {
		t.Errorf("expected provider 'memory', got '%s'", output.Provider)
	}

	if output.Count != 3 {
		t.Errorf("expected 3 indices, got %d", output.Count)
	}
}

func TestSearch(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Index some documents
	docs := []map[string]interface{}{
		{"title": "Hello World", "content": "This is a test document"},
		{"title": "Go Programming", "content": "Learn Go programming language"},
		{"title": "Test Document", "content": "Another test document for testing"},
	}
	for i, doc := range docs {
		err := provider.Index(ctx, "test-index", fmt.Sprintf("doc-%d", i), doc)
		if err != nil {
			t.Fatalf("failed to index: %v", err)
		}
	}

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find search_query tool
	var searchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_query" {
			searchTool = tl
			break
		}
	}

	if searchTool == nil {
		t.Fatal("search_query tool not found")
	}

	// Execute search
	input := json.RawMessage(`{"index": "test-index", "query": "test"}`)
	result, err := searchTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("search_query failed: %v", err)
	}

	var output struct {
		Index     string `json:"index"`
		Query     string `json:"query"`
		TotalHits int64  `json:"total_hits"`
		Hits      []struct {
			ID     string                 `json:"id"`
			Source map[string]interface{} `json:"source"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.TotalHits != 2 {
		t.Errorf("expected 2 hits, got %d", output.TotalHits)
	}
}

func TestSearch_MissingIndex(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var searchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_query" {
			searchTool = tl
			break
		}
	}

	input := json.RawMessage(`{"query": "test"}`)
	_, err = searchTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestSearch_MissingQuery(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var searchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_query" {
			searchTool = tl
			break
		}
	}

	input := json.RawMessage(`{"index": "test-index"}`)
	_, err = searchTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestSearch_WithHighlight(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	err := provider.Index(ctx, "test-index", "doc-1", map[string]interface{}{
		"title":   "Hello World",
		"content": "This is a test document",
	})
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var searchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_query" {
			searchTool = tl
			break
		}
	}

	input := json.RawMessage(`{"index": "test-index", "query": "test", "highlight": true}`)
	result, err := searchTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("search_query failed: %v", err)
	}

	var output struct {
		Hits []struct {
			Highlights map[string][]string `json:"highlights,omitempty"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(output.Hits) == 0 {
		t.Fatal("expected at least one hit")
	}

	if len(output.Hits[0].Highlights) == 0 {
		t.Error("expected highlights in result")
	}
}

func TestGetDocument(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Index a document
	doc := map[string]interface{}{
		"title":   "Test Document",
		"content": "Document content",
	}
	err := provider.Index(ctx, "test-index", "doc-1", doc)
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find search_get_document tool
	var getDocTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_get_document" {
			getDocTool = tl
			break
		}
	}

	if getDocTool == nil {
		t.Fatal("search_get_document tool not found")
	}

	// Get document
	input := json.RawMessage(`{"index": "test-index", "id": "doc-1"}`)
	result, err := getDocTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("search_get_document failed: %v", err)
	}

	var output struct {
		Index    string                 `json:"index"`
		ID       string                 `json:"id"`
		Found    bool                   `json:"found"`
		Document map[string]interface{} `json:"document"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Found {
		t.Error("expected document to be found")
	}

	if output.Document["title"] != "Test Document" {
		t.Errorf("expected title 'Test Document', got '%v'", output.Document["title"])
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	provider.CreateIndex("test-index")

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var getDocTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_get_document" {
			getDocTool = tl
			break
		}
	}

	input := json.RawMessage(`{"index": "test-index", "id": "nonexistent"}`)
	result, err := getDocTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("search_get_document failed: %v", err)
	}

	var output struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Found {
		t.Error("expected document not to be found")
	}
}

func TestGetDocument_MissingIndex(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var getDocTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_get_document" {
			getDocTool = tl
			break
		}
	}

	input := json.RawMessage(`{"id": "doc-1"}`)
	_, err = getDocTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestGetDocument_MissingID(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var getDocTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_get_document" {
			getDocTool = tl
			break
		}
	}

	input := json.RawMessage(`{"index": "test-index"}`)
	_, err = getDocTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestIndexDocument(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find search_index_document tool
	var indexTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_index_document" {
			indexTool = tl
			break
		}
	}

	if indexTool == nil {
		t.Fatal("search_index_document tool not found")
	}

	// Index a document
	input := json.RawMessage(`{"index": "test-index", "id": "doc-1", "document": {"title": "Test", "content": "Content"}}`)
	result, err := indexTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("search_index_document failed: %v", err)
	}

	var output struct {
		Index   string `json:"index"`
		ID      string `json:"id"`
		Created bool   `json:"created"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Created {
		t.Error("expected created to be true")
	}

	// Verify document was stored
	if provider.DocumentCount("test-index") != 1 {
		t.Errorf("expected 1 document, got %d", provider.DocumentCount("test-index"))
	}
}

func TestIndexDocument_MissingIndex(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var indexTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_index_document" {
			indexTool = tl
			break
		}
	}

	input := json.RawMessage(`{"document": {"title": "Test"}}`)
	_, err = indexTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestIndexDocument_MissingDocument(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var indexTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_index_document" {
			indexTool = tl
			break
		}
	}

	input := json.RawMessage(`{"index": "test-index"}`)
	_, err = indexTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing document")
	}
}

func TestDeleteDocument(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Index a document
	err := provider.Index(ctx, "test-index", "doc-1", map[string]interface{}{"title": "Test"})
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	pack, err := search.New(provider, search.WithWriteAccess(), search.WithDeleteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find search_delete_document tool
	var deleteTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_delete_document" {
			deleteTool = tl
			break
		}
	}

	if deleteTool == nil {
		t.Fatal("search_delete_document tool not found")
	}

	// Delete document
	input := json.RawMessage(`{"index": "test-index", "id": "doc-1"}`)
	result, err := deleteTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("search_delete_document failed: %v", err)
	}

	var output struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Deleted {
		t.Error("expected deleted to be true")
	}

	// Verify document was deleted
	if provider.DocumentCount("test-index") != 0 {
		t.Errorf("expected 0 documents, got %d", provider.DocumentCount("test-index"))
	}
}

func TestContextCancelled(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var listIndicesTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_list_indices" {
			listIndicesTool = tl
			break
		}
	}

	_, err = listIndicesTool.Execute(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWithTimeout(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	pack, err := search.New(provider, search.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}
}

func TestWithMaxResultSize(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Index many documents
	for i := 0; i < 20; i++ {
		err := provider.Index(ctx, "test-index", fmt.Sprintf("doc-%d", i), map[string]interface{}{
			"content": "test document",
		})
		if err != nil {
			t.Fatalf("failed to index: %v", err)
		}
	}

	// Create pack with limited result size
	pack, err := search.New(provider, search.WithMaxResultSize(5))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var searchTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "search_query" {
			searchTool = tl
			break
		}
	}

	// Request more than max
	input := json.RawMessage(`{"index": "test-index", "query": "test", "size": 100}`)
	result, err := searchTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("search_query failed: %v", err)
	}

	var output struct {
		Hits []interface{} `json:"hits"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should be limited to max
	if len(output.Hits) != 5 {
		t.Errorf("expected 5 hits (max), got %d", len(output.Hits))
	}
}

func TestMemoryProvider_IndexExists(t *testing.T) {
	provider := search.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Index doesn't exist
	exists, err := provider.IndexExists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("IndexExists failed: %v", err)
	}
	if exists {
		t.Error("expected index not to exist")
	}

	// Create index
	provider.CreateIndex("test-index")

	// Index exists
	exists, err = provider.IndexExists(ctx, "test-index")
	if err != nil {
		t.Fatalf("IndexExists failed: %v", err)
	}
	if !exists {
		t.Error("expected index to exist")
	}
}
