package image

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider:       provider,
		DefaultQuality: 90,
	})

	if p.Name != "image" {
		t.Errorf("expected pack name 'image', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"image_resize", "image_convert", "image_analyze"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestResize(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful resize",
			input: map[string]interface{}{
				"source": "base64imagedata",
				"width":  100,
				"height": 100,
			},
			wantErr: false,
		},
		{
			name: "resize with all options",
			input: map[string]interface{}{
				"source":        "base64imagedata",
				"source_type":   "base64",
				"width":         200,
				"height":        150,
				"mode":          "fill",
				"quality":       95,
				"output_format": "jpeg",
			},
			wantErr: false,
		},
		{
			name: "resize from URL",
			input: map[string]interface{}{
				"source":      "https://example.com/image.png",
				"source_type": "url",
				"width":       300,
				"height":      200,
			},
			wantErr: false,
		},
		{
			name: "missing source returns error",
			input: map[string]interface{}{
				"width":  100,
				"height": 100,
			},
			wantErr: true,
		},
		{
			name: "invalid width returns error",
			input: map[string]interface{}{
				"source": "base64data",
				"width":  0,
				"height": 100,
			},
			wantErr: true,
		},
		{
			name: "invalid height returns error",
			input: map[string]interface{}{
				"source": "base64data",
				"width":  100,
				"height": -1,
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"source": "base64data",
				"width":  100,
				"height": 100,
			},
			setupFunc: func(p *MockProvider) {
				p.ResizeFunc = func(context.Context, ResizeRequest) (ResizeResponse, error) {
					return ResizeResponse{}, errors.New("resize error")
				}
			},
			wantErr:     true,
			errContains: "resize error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:       provider,
				DefaultQuality: 85,
			})

			var resizeTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "image_resize" {
					resizeTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := resizeTool.Execute(context.Background(), input)

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

			var resp ResizeResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.Data == "" {
				t.Error("expected non-empty image data")
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful convert to jpeg",
			input: map[string]interface{}{
				"source":        "base64imagedata",
				"target_format": "jpeg",
			},
			wantErr: false,
		},
		{
			name: "convert with all options",
			input: map[string]interface{}{
				"source":         "base64imagedata",
				"source_type":    "base64",
				"target_format":  "webp",
				"quality":        90,
				"strip_metadata": true,
			},
			wantErr: false,
		},
		{
			name: "convert to png",
			input: map[string]interface{}{
				"source":        "base64data",
				"target_format": "png",
			},
			wantErr: false,
		},
		{
			name: "missing source returns error",
			input: map[string]interface{}{
				"target_format": "jpeg",
			},
			wantErr: true,
		},
		{
			name: "missing target format returns error",
			input: map[string]interface{}{
				"source": "base64data",
			},
			wantErr: true,
		},
		{
			name: "unsupported format returns error",
			input: map[string]interface{}{
				"source":        "base64data",
				"target_format": "tiff",
			},
			setupFunc: func(p *MockProvider) {
				p.ConvertFunc = func(_ context.Context, req ConvertRequest) (ConvertResponse, error) {
					if req.TargetFormat == "tiff" {
						return ConvertResponse{}, ErrUnsupportedFormat
					}
					return ConvertResponse{}, nil
				}
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"source":        "base64data",
				"target_format": "jpeg",
			},
			setupFunc: func(p *MockProvider) {
				p.ConvertFunc = func(context.Context, ConvertRequest) (ConvertResponse, error) {
					return ConvertResponse{}, errors.New("convert error")
				}
			},
			wantErr:     true,
			errContains: "convert error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:       provider,
				DefaultQuality: 85,
			})

			var convertTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "image_convert" {
					convertTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := convertTool.Execute(context.Background(), input)

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

			var resp ConvertResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.Data == "" {
				t.Error("expected non-empty image data")
			}
		})
	}
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful analyze",
			input: map[string]interface{}{
				"source": "base64imagedata",
			},
			wantErr: false,
		},
		{
			name: "analyze with metadata",
			input: map[string]interface{}{
				"source":           "base64imagedata",
				"include_metadata": true,
			},
			wantErr: false,
		},
		{
			name: "analyze with specific features",
			input: map[string]interface{}{
				"source":   "base64imagedata",
				"features": []string{"colors", "labels", "faces"},
			},
			wantErr: false,
		},
		{
			name: "analyze from URL",
			input: map[string]interface{}{
				"source":      "https://example.com/image.png",
				"source_type": "url",
			},
			wantErr: false,
		},
		{
			name: "missing source returns error",
			input: map[string]interface{}{
				"features": []string{"colors"},
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"source": "base64data",
			},
			setupFunc: func(p *MockProvider) {
				p.AnalyzeFunc = func(context.Context, AnalyzeRequest) (AnalyzeResponse, error) {
					return AnalyzeResponse{}, errors.New("analyze error")
				}
			},
			wantErr:     true,
			errContains: "analyze error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider: provider,
			})

			var analyzeTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "image_analyze" {
					analyzeTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := analyzeTool.Execute(context.Background(), input)

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

			var resp AnalyzeResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.Width == 0 || resp.Height == 0 {
				t.Error("expected non-zero dimensions")
			}

			// Analyze results should be cacheable
			if !result.Cached {
				t.Error("expected analyze results to be cached")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"source":        "base64data",
			"width":         100,
			"height":        100,
			"target_format": "png",
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

	// Test Resize
	resizeResp, err := provider.Resize(ctx, ResizeRequest{
		Source: "base64data",
		Width:  200,
		Height: 150,
	})
	if err != nil {
		t.Errorf("Resize error: %v", err)
	}
	if resizeResp.Width != 200 || resizeResp.Height != 150 {
		t.Errorf("expected 200x150, got %dx%d", resizeResp.Width, resizeResp.Height)
	}
	if resizeResp.Data == "" {
		t.Error("expected non-empty data")
	}

	// Test Convert
	convertResp, err := provider.Convert(ctx, ConvertRequest{
		Source:       "base64data",
		TargetFormat: "jpeg",
	})
	if err != nil {
		t.Errorf("Convert error: %v", err)
	}
	if convertResp.Format != "jpeg" {
		t.Errorf("expected format jpeg, got %s", convertResp.Format)
	}

	// Test Analyze
	analyzeResp, err := provider.Analyze(ctx, AnalyzeRequest{
		Source:          "base64data",
		IncludeMetadata: true,
		Features:        []string{"colors", "labels", "faces", "text"},
	})
	if err != nil {
		t.Errorf("Analyze error: %v", err)
	}
	if analyzeResp.Width == 0 {
		t.Error("expected non-zero width")
	}
	if analyzeResp.Metadata == nil {
		t.Error("expected metadata to be included")
	}
	if len(analyzeResp.DominantColors) == 0 {
		t.Error("expected dominant colors")
	}
	if len(analyzeResp.Labels) == 0 {
		t.Error("expected labels")
	}
	if len(analyzeResp.Faces) == 0 {
		t.Error("expected faces")
	}
	if len(analyzeResp.Text) == 0 {
		t.Error("expected text regions")
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultQuality != 85 {
		t.Errorf("expected DefaultQuality 85, got %d", cfg.DefaultQuality)
	}

	if cfg.MaxImageSize != 50*1024*1024 {
		t.Errorf("expected MaxImageSize 50MB, got %d", cfg.MaxImageSize)
	}
}

func TestGenerateMockImageData(t *testing.T) {
	tests := []struct {
		format   string
		expected []byte
	}{
		{"png", []byte{0x89, 0x50, 0x4E, 0x47}},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}},
		{"gif", []byte{0x47, 0x49, 0x46, 0x38}},
		{"webp", []byte{0x52, 0x49, 0x46, 0x46}},
		{"bmp", []byte{0x42, 0x4D}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			data := generateMockImageData(100, 100, tt.format)
			if len(data) < len(tt.expected) {
				t.Fatalf("data too short: %d", len(data))
			}
			for i, b := range tt.expected {
				if data[i] != b {
					t.Errorf("byte %d: expected %x, got %x", i, b, data[i])
				}
			}
		})
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
