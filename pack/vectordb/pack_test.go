package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider:          provider,
		DefaultCollection: "test-collection",
		DefaultTopK:       5,
	})

	if p.Name != "vectordb" {
		t.Errorf("expected pack name 'vectordb', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"vector_upsert", "vector_query", "vector_delete"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestUpsert(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful upsert",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vectors": []map[string]interface{}{
					{
						"id":     "vec-1",
						"values": []float64{0.1, 0.2, 0.3},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "upsert with metadata",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vectors": []map[string]interface{}{
					{
						"id":       "vec-1",
						"values":   []float64{0.1, 0.2, 0.3},
						"metadata": map[string]interface{}{"key": "value"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "upsert uses default collection",
			input: map[string]interface{}{
				"vectors": []map[string]interface{}{
					{
						"id":     "vec-1",
						"values": []float64{0.1, 0.2, 0.3},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty vectors returns error",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vectors":    []map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "missing collection without default returns error",
			input: map[string]interface{}{
				"vectors": []map[string]interface{}{
					{
						"id":     "vec-1",
						"values": []float64{0.1, 0.2, 0.3},
					},
				},
			},
			setupFunc: func(p *MockProvider) {
				// Will use a config without default collection
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vectors": []map[string]interface{}{
					{
						"id":     "vec-1",
						"values": []float64{0.1, 0.2, 0.3},
					},
				},
			},
			setupFunc: func(p *MockProvider) {
				p.UpsertFunc = func(context.Context, UpsertRequest) (UpsertResponse, error) {
					return UpsertResponse{}, errors.New("provider error")
				}
			},
			wantErr:     true,
			errContains: "provider error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			defaultCollection := "default-collection"
			if tt.name == "missing collection without default returns error" {
				defaultCollection = ""
			}

			p := New(PackConfig{
				Provider:          provider,
				DefaultCollection: defaultCollection,
			})

			var upsertTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "vector_upsert" {
					upsertTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := upsertTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp UpsertResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.UpsertedCount == 0 {
				t.Error("expected non-zero upserted count")
			}
		})
	}
}

func TestQuery(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful query",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vector":     []float64{0.1, 0.2, 0.3},
				"top_k":      5,
			},
			wantErr: false,
		},
		{
			name: "query with metadata filter",
			input: map[string]interface{}{
				"collection":       "test-collection",
				"vector":           []float64{0.1, 0.2, 0.3},
				"include_metadata": true,
				"include_values":   true,
			},
			wantErr: false,
		},
		{
			name: "query uses default collection",
			input: map[string]interface{}{
				"vector": []float64{0.1, 0.2, 0.3},
			},
			wantErr: false,
		},
		{
			name: "query uses default top_k",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vector":     []float64{0.1, 0.2, 0.3},
			},
			wantErr: false,
		},
		{
			name: "empty vector returns error",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vector":     []float64{},
			},
			wantErr: true,
		},
		{
			name: "missing vector returns error",
			input: map[string]interface{}{
				"collection": "test-collection",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"collection": "test-collection",
				"vector":     []float64{0.1, 0.2, 0.3},
			},
			setupFunc: func(p *MockProvider) {
				p.QueryFunc = func(context.Context, QueryRequest) (QueryResponse, error) {
					return QueryResponse{}, errors.New("query error")
				}
			},
			wantErr:     true,
			errContains: "query error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")

			// Pre-populate with some vectors
			_, _ = provider.Upsert(context.Background(), UpsertRequest{
				Collection: "test-collection",
				Vectors: []Vector{
					{ID: "vec-1", Values: []float64{0.1, 0.2, 0.3}},
					{ID: "vec-2", Values: []float64{0.4, 0.5, 0.6}},
				},
			})
			_, _ = provider.Upsert(context.Background(), UpsertRequest{
				Collection: "default-collection",
				Vectors: []Vector{
					{ID: "vec-1", Values: []float64{0.1, 0.2, 0.3}},
				},
			})

			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:          provider,
				DefaultCollection: "default-collection",
				DefaultTopK:       10,
			})

			var queryTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "vector_query" {
					queryTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := queryTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp QueryResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			// Query results should be cacheable
			if !result.Cached {
				t.Error("expected query results to be cached")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful delete by IDs",
			input: map[string]interface{}{
				"collection": "test-collection",
				"ids":        []string{"vec-1", "vec-2"},
			},
			wantErr: false,
		},
		{
			name: "delete all in collection",
			input: map[string]interface{}{
				"collection": "test-collection",
				"delete_all": true,
			},
			wantErr: false,
		},
		{
			name: "delete with filter",
			input: map[string]interface{}{
				"collection": "test-collection",
				"filter":     map[string]interface{}{"category": "test"},
			},
			wantErr: false,
		},
		{
			name: "delete uses default collection",
			input: map[string]interface{}{
				"ids": []string{"vec-1"},
			},
			wantErr: false,
		},
		{
			name: "missing ids and not delete_all returns error",
			input: map[string]interface{}{
				"collection": "test-collection",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"collection": "test-collection",
				"ids":        []string{"vec-1"},
			},
			setupFunc: func(p *MockProvider) {
				p.DeleteFunc = func(context.Context, DeleteRequest) (DeleteResponse, error) {
					return DeleteResponse{}, errors.New("delete error")
				}
			},
			wantErr:     true,
			errContains: "delete error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")

			// Pre-populate with some vectors
			_, _ = provider.Upsert(context.Background(), UpsertRequest{
				Collection: "test-collection",
				Vectors: []Vector{
					{ID: "vec-1", Values: []float64{0.1, 0.2, 0.3}},
					{ID: "vec-2", Values: []float64{0.4, 0.5, 0.6}},
				},
			})
			_, _ = provider.Upsert(context.Background(), UpsertRequest{
				Collection: "default-collection",
				Vectors: []Vector{
					{ID: "vec-1", Values: []float64{0.1, 0.2, 0.3}},
				},
			})

			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:          provider,
				DefaultCollection: "default-collection",
			})

			var deleteTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "vector_delete" {
					deleteTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := deleteTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp DeleteResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"collection": "test",
			"vectors": []map[string]interface{}{
				{"id": "1", "values": []float64{0.1}},
			},
			"vector":     []float64{0.1},
			"ids":        []string{"1"},
			"delete_all": true,
		})

		_, err := tool.Execute(context.Background(), input)
		if !errors.Is(err, ErrProviderNotConfigured) {
			t.Errorf("tool %s: expected ErrProviderNotConfigured, got %v", tool.Name(), err)
		}
	}
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("test-mock")

	if provider.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %s", provider.Name())
	}

	ctx := context.Background()

	// Test Available
	if !provider.Available(ctx) {
		t.Error("expected provider to be available")
	}

	// Test Upsert
	upsertResp, err := provider.Upsert(ctx, UpsertRequest{
		Collection: "test",
		Vectors: []Vector{
			{ID: "vec-1", Values: []float64{1.0, 0.0, 0.0}, Metadata: map[string]interface{}{"type": "a"}},
			{ID: "vec-2", Values: []float64{0.0, 1.0, 0.0}, Metadata: map[string]interface{}{"type": "b"}},
			{ID: "vec-3", Values: []float64{0.0, 0.0, 1.0}, Metadata: map[string]interface{}{"type": "c"}},
		},
	})
	if err != nil {
		t.Errorf("Upsert error: %v", err)
	}
	if upsertResp.UpsertedCount != 3 {
		t.Errorf("expected 3 upserted, got %d", upsertResp.UpsertedCount)
	}

	// Test Query - should find vec-1 as most similar
	queryResp, err := provider.Query(ctx, QueryRequest{
		Collection:      "test",
		Vector:          []float64{0.9, 0.1, 0.0},
		TopK:            2,
		IncludeMetadata: true,
		IncludeValues:   true,
	})
	if err != nil {
		t.Errorf("Query error: %v", err)
	}
	if len(queryResp.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(queryResp.Matches))
	}
	if queryResp.Matches[0].ID != "vec-1" {
		t.Errorf("expected first match to be vec-1, got %s", queryResp.Matches[0].ID)
	}
	if queryResp.Matches[0].Metadata == nil {
		t.Error("expected metadata to be included")
	}
	if queryResp.Matches[0].Values == nil {
		t.Error("expected values to be included")
	}

	// Test Delete
	deleteResp, err := provider.Delete(ctx, DeleteRequest{
		Collection: "test",
		IDs:        []string{"vec-1"},
	})
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}
	if deleteResp.DeletedCount != 1 {
		t.Errorf("expected 1 deleted, got %d", deleteResp.DeletedCount)
	}

	// Verify vec-1 is gone
	queryResp, err = provider.Query(ctx, QueryRequest{
		Collection: "test",
		Vector:     []float64{1.0, 0.0, 0.0},
		TopK:       10,
	})
	if err != nil {
		t.Errorf("Query error: %v", err)
	}
	for _, m := range queryResp.Matches {
		if m.ID == "vec-1" {
			t.Error("vec-1 should have been deleted")
		}
	}

	// Test Delete All
	deleteResp, err = provider.Delete(ctx, DeleteRequest{
		Collection: "test",
		DeleteAll:  true,
	})
	if err != nil {
		t.Errorf("Delete all error: %v", err)
	}
	if deleteResp.DeletedCount != 2 {
		t.Errorf("expected 2 deleted, got %d", deleteResp.DeletedCount)
	}

	// Verify collection is empty
	queryResp, err = provider.Query(ctx, QueryRequest{
		Collection: "test",
		Vector:     []float64{1.0, 0.0, 0.0},
		TopK:       10,
	})
	if err != nil {
		t.Errorf("Query error: %v", err)
	}
	if len(queryResp.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(queryResp.Matches))
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1.0, 0.0, 0.0},
			b:        []float64{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "different lengths returns zero",
			a:        []float64{1.0, 0.0},
			b:        []float64{1.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "zero vector returns zero",
			a:        []float64{0.0, 0.0, 0.0},
			b:        []float64{1.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if !floatEquals(result, tt.expected, 0.0001) {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultTopK != 10 {
		t.Errorf("expected DefaultTopK 10, got %d", cfg.DefaultTopK)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func floatEquals(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}
