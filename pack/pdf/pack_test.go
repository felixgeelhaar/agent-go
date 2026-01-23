package pdf

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider: provider,
	})

	if p.Name != "pdf" {
		t.Errorf("expected pack name 'pdf', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"pdf_generate", "pdf_parse", "pdf_merge"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful generate from content",
			input: map[string]interface{}{
				"content": "Hello, World!",
				"format":  "text",
			},
			wantErr: false,
		},
		{
			name: "generate with all options",
			input: map[string]interface{}{
				"content":     "<h1>Title</h1><p>Content</p>",
				"format":      "html",
				"page_size":   "Letter",
				"orientation": "landscape",
				"header":      "Page Header",
				"footer":      "Page Footer",
				"watermark":   "DRAFT",
				"metadata": map[string]interface{}{
					"title":  "Test Document",
					"author": "Test Author",
				},
			},
			wantErr: false,
		},
		{
			name: "generate from template",
			input: map[string]interface{}{
				"template": "invoice",
				"variables": map[string]interface{}{
					"customer": "John Doe",
					"amount":   100.50,
				},
			},
			setupFunc: func(p *MockProvider) {
				p.AddTemplate("invoice", "Invoice for {{customer}}: ${{amount}}")
			},
			wantErr: false,
		},
		{
			name: "missing content and template returns error",
			input: map[string]interface{}{
				"format": "text",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"content": "test",
			},
			setupFunc: func(p *MockProvider) {
				p.GenerateFunc = func(context.Context, GenerateRequest) (GenerateResponse, error) {
					return GenerateResponse{}, errors.New("generate error")
				}
			},
			wantErr:     true,
			errContains: "generate error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var generateTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "pdf_generate" {
					generateTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := generateTool.Execute(context.Background(), input)

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

			var resp GenerateResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}

			if resp.Content == "" {
				t.Error("expected non-empty content")
			}

			if resp.PageCount < 1 {
				t.Error("expected at least 1 page")
			}
		})
	}
}

func TestParse(t *testing.T) {
	// Create a mock PDF content
	mockPDF := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nThis is test content\n%%EOF"))

	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful parse",
			input: map[string]interface{}{
				"content": mockPDF,
			},
			wantErr: false,
		},
		{
			name: "parse with image extraction",
			input: map[string]interface{}{
				"content":        mockPDF,
				"extract_images": true,
			},
			wantErr: false,
		},
		{
			name: "parse with table extraction",
			input: map[string]interface{}{
				"content":        mockPDF,
				"extract_tables": true,
			},
			wantErr: false,
		},
		{
			name: "parse with page range",
			input: map[string]interface{}{
				"content":    mockPDF,
				"page_range": "1-5",
			},
			wantErr: false,
		},
		{
			name: "missing content returns error",
			input: map[string]interface{}{
				"extract_images": true,
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"content": mockPDF,
			},
			setupFunc: func(p *MockProvider) {
				p.ParseFunc = func(context.Context, ParseRequest) (ParseResponse, error) {
					return ParseResponse{}, errors.New("parse error")
				}
			},
			wantErr:     true,
			errContains: "parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var parseTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "pdf_parse" {
					parseTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := parseTool.Execute(context.Background(), input)

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

			var resp ParseResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.PageCount < 1 {
				t.Error("expected at least 1 page")
			}
		})
	}
}

func TestMerge(t *testing.T) {
	// Create mock PDF contents
	mockPDF1 := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nDocument 1 content\n%%EOF"))
	mockPDF2 := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nDocument 2 content\n%%EOF"))

	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful merge",
			input: map[string]interface{}{
				"documents": []string{mockPDF1, mockPDF2},
			},
			wantErr: false,
		},
		{
			name: "merge with bookmarks",
			input: map[string]interface{}{
				"documents":       []string{mockPDF1, mockPDF2},
				"bookmarks":       true,
				"bookmark_titles": []string{"Document 1", "Document 2"},
			},
			wantErr: false,
		},
		{
			name: "less than 2 documents returns error",
			input: map[string]interface{}{
				"documents": []string{mockPDF1},
			},
			wantErr: true,
		},
		{
			name: "empty documents returns error",
			input: map[string]interface{}{
				"documents": []string{},
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"documents": []string{mockPDF1, mockPDF2},
			},
			setupFunc: func(p *MockProvider) {
				p.MergeFunc = func(context.Context, MergeRequest) (MergeResponse, error) {
					return MergeResponse{}, errors.New("merge error")
				}
			},
			wantErr:     true,
			errContains: "merge error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var mergeTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "pdf_merge" {
					mergeTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := mergeTool.Execute(context.Background(), input)

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

			var resp MergeResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}

			if resp.Content == "" {
				t.Error("expected non-empty content")
			}

			if resp.PageCount < 2 {
				t.Error("expected at least 2 pages after merge")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	mockPDF := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nTest\n%%EOF"))

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"content":   "test",
			"documents": []string{mockPDF, mockPDF},
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

	// Test Generate
	generateResp, err := provider.Generate(ctx, GenerateRequest{
		Content: "Test content",
		Format:  "text",
	})
	if err != nil {
		t.Errorf("Generate error: %v", err)
	}
	if !generateResp.Success {
		t.Error("expected success")
	}
	if generateResp.Content == "" {
		t.Error("expected content")
	}

	// Test Parse
	parseResp, err := provider.Parse(ctx, ParseRequest{
		Content:       generateResp.Content,
		ExtractImages: true,
		ExtractTables: true,
	})
	if err != nil {
		t.Errorf("Parse error: %v", err)
	}
	if parseResp.PageCount < 1 {
		t.Error("expected at least 1 page")
	}
	if len(parseResp.Images) == 0 {
		t.Error("expected images to be extracted")
	}
	if len(parseResp.Tables) == 0 {
		t.Error("expected tables to be extracted")
	}

	// Test Merge
	mergeResp, err := provider.Merge(ctx, MergeRequest{
		Documents: []string{generateResp.Content, generateResp.Content},
		Bookmarks: true,
	})
	if err != nil {
		t.Errorf("Merge error: %v", err)
	}
	if !mergeResp.Success {
		t.Error("expected success")
	}

	// Verify generated count
	if provider.GeneratedCount() != 1 {
		t.Errorf("expected 1 generated PDF, got %d", provider.GeneratedCount())
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultPageSize != "A4" {
		t.Errorf("expected DefaultPageSize 'A4', got %s", cfg.DefaultPageSize)
	}

	if cfg.DefaultOrientation != "portrait" {
		t.Errorf("expected DefaultOrientation 'portrait', got %s", cfg.DefaultOrientation)
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
