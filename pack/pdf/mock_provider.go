package pdf

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
)

// MockProvider is a mock PDF provider for testing.
type MockProvider struct {
	name string

	// GenerateFunc is called when Generate is invoked.
	GenerateFunc func(ctx context.Context, req GenerateRequest) (GenerateResponse, error)

	// ParseFunc is called when Parse is invoked.
	ParseFunc func(ctx context.Context, req ParseRequest) (ParseResponse, error)

	// MergeFunc is called when Merge is invoked.
	MergeFunc func(ctx context.Context, req MergeRequest) (MergeResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool

	// Internal state
	mu            sync.RWMutex
	generatedPDFs []generatedPDF
	templates     map[string]string
}

type generatedPDF struct {
	content   string
	format    string
	pageCount int
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name:          name,
		generatedPDFs: make([]generatedPDF, 0),
		templates:     make(map[string]string),
	}

	p.GenerateFunc = p.defaultGenerate
	p.ParseFunc = p.defaultParse
	p.MergeFunc = p.defaultMerge
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Generate generates a PDF.
func (p *MockProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	return p.GenerateFunc(ctx, req)
}

// Parse parses a PDF.
func (p *MockProvider) Parse(ctx context.Context, req ParseRequest) (ParseResponse, error) {
	return p.ParseFunc(ctx, req)
}

// Merge merges PDFs.
func (p *MockProvider) Merge(ctx context.Context, req MergeRequest) (MergeResponse, error) {
	return p.MergeFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultGenerate(_ context.Context, req GenerateRequest) (GenerateResponse, error) {
	if req.Content == "" && req.Template == "" {
		return GenerateResponse{}, ErrInvalidInput
	}

	content := req.Content
	if req.Template != "" {
		p.mu.RLock()
		tmpl, ok := p.templates[req.Template]
		p.mu.RUnlock()
		if !ok {
			return GenerateResponse{}, ErrInvalidInput
		}
		content = tmpl
		// Simple variable substitution
		for key, value := range req.Variables {
			placeholder := fmt.Sprintf("{{%s}}", key)
			content = strings.ReplaceAll(content, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// Mock PDF generation - create a simple base64 encoded "PDF"
	// In reality, this would use a PDF library
	mockPDF := fmt.Sprintf("%%PDF-1.4\n%s\n%%%%EOF", content)
	encoded := base64.StdEncoding.EncodeToString([]byte(mockPDF))

	// Estimate page count based on content length
	pageCount := 1 + len(content)/3000

	p.mu.Lock()
	p.generatedPDFs = append(p.generatedPDFs, generatedPDF{
		content:   content,
		format:    req.Format,
		pageCount: pageCount,
	})
	p.mu.Unlock()

	return GenerateResponse{
		Content:   encoded,
		Size:      int64(len(encoded)),
		PageCount: pageCount,
		Success:   true,
	}, nil
}

func (p *MockProvider) defaultParse(_ context.Context, req ParseRequest) (ParseResponse, error) {
	if req.Content == "" {
		return ParseResponse{}, ErrInvalidInput
	}

	// Decode base64 content
	decoded, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		return ParseResponse{}, ErrParseFailed
	}

	content := string(decoded)

	// Extract text between PDF markers
	text := content
	if idx := strings.Index(content, "\n"); idx > 0 {
		if endIdx := strings.LastIndex(content, "\n"); endIdx > idx {
			text = strings.TrimSpace(content[idx:endIdx])
		}
	}

	// Create mock pages
	pages := []PageContent{
		{
			Number: 1,
			Text:   text,
			Width:  612,  // Letter size width in points
			Height: 792,  // Letter size height in points
		},
	}

	resp := ParseResponse{
		Text:      text,
		Pages:     pages,
		PageCount: len(pages),
		Metadata: Metadata{
			Title:   "Mock PDF",
			Creator: "MockProvider",
		},
	}

	// Mock image extraction
	if req.ExtractImages {
		resp.Images = []ExtractedImage{
			{
				Page:    1,
				Content: base64.StdEncoding.EncodeToString([]byte("mock-image-data")),
				Format:  "png",
				Width:   100,
				Height:  100,
			},
		}
	}

	// Mock table extraction
	if req.ExtractTables {
		resp.Tables = []ExtractedTable{
			{
				Page:    1,
				Headers: []string{"Column1", "Column2"},
				Rows: [][]string{
					{"Value1", "Value2"},
					{"Value3", "Value4"},
				},
			},
		}
	}

	return resp, nil
}

func (p *MockProvider) defaultMerge(_ context.Context, req MergeRequest) (MergeResponse, error) {
	if len(req.Documents) < 2 {
		return MergeResponse{}, ErrInvalidInput
	}

	// Decode and concatenate content
	var mergedContent strings.Builder
	mergedContent.WriteString("%PDF-1.4\n")

	totalPages := 0
	for i, doc := range req.Documents {
		decoded, err := base64.StdEncoding.DecodeString(doc)
		if err != nil {
			return MergeResponse{}, ErrMergeFailed
		}

		content := string(decoded)
		// Strip PDF headers/footers and extract content
		if idx := strings.Index(content, "\n"); idx > 0 {
			if endIdx := strings.LastIndex(content, "\n"); endIdx > idx {
				content = strings.TrimSpace(content[idx:endIdx])
			}
		}

		if req.Bookmarks && i < len(req.BookmarkTitles) {
			mergedContent.WriteString(fmt.Sprintf("[Bookmark: %s]\n", req.BookmarkTitles[i]))
		}
		mergedContent.WriteString(content)
		mergedContent.WriteString("\n---PAGE BREAK---\n")
		totalPages++
	}

	mergedContent.WriteString("%%EOF")

	encoded := base64.StdEncoding.EncodeToString([]byte(mergedContent.String()))

	return MergeResponse{
		Content:   encoded,
		Size:      int64(len(encoded)),
		PageCount: totalPages,
		Success:   true,
	}, nil
}

// AddTemplate adds a template for testing.
func (p *MockProvider) AddTemplate(name, content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templates[name] = content
}

// GeneratedCount returns the number of generated PDFs (for testing).
func (p *MockProvider) GeneratedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.generatedPDFs)
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
