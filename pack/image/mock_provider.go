package image

import (
	"context"
	"encoding/base64"
)

// MockProvider is a mock image provider for testing.
type MockProvider struct {
	name string

	// ResizeFunc is called when Resize is invoked.
	ResizeFunc func(ctx context.Context, req ResizeRequest) (ResizeResponse, error)

	// ConvertFunc is called when Convert is invoked.
	ConvertFunc func(ctx context.Context, req ConvertRequest) (ConvertResponse, error)

	// AnalyzeFunc is called when Analyze is invoked.
	AnalyzeFunc func(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name: name,
	}

	p.ResizeFunc = p.defaultResize
	p.ConvertFunc = p.defaultConvert
	p.AnalyzeFunc = p.defaultAnalyze
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Resize resizes an image.
func (p *MockProvider) Resize(ctx context.Context, req ResizeRequest) (ResizeResponse, error) {
	return p.ResizeFunc(ctx, req)
}

// Convert converts an image format.
func (p *MockProvider) Convert(ctx context.Context, req ConvertRequest) (ConvertResponse, error) {
	return p.ConvertFunc(ctx, req)
}

// Analyze analyzes an image.
func (p *MockProvider) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	return p.AnalyzeFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultResize(_ context.Context, req ResizeRequest) (ResizeResponse, error) {
	if req.Source == "" {
		return ResizeResponse{}, ErrInvalidInput
	}

	if req.Width <= 0 || req.Height <= 0 {
		return ResizeResponse{}, ErrInvalidInput
	}

	// Generate mock resized image data
	outputFormat := req.OutputFormat
	if outputFormat == "" {
		outputFormat = "png"
	}

	mockData := generateMockImageData(req.Width, req.Height, outputFormat)

	return ResizeResponse{
		Data:   base64.StdEncoding.EncodeToString(mockData),
		Format: outputFormat,
		Width:  req.Width,
		Height: req.Height,
		Size:   int64(len(mockData)),
	}, nil
}

func (p *MockProvider) defaultConvert(_ context.Context, req ConvertRequest) (ConvertResponse, error) {
	if req.Source == "" {
		return ConvertResponse{}, ErrInvalidInput
	}

	if req.TargetFormat == "" {
		return ConvertResponse{}, ErrInvalidInput
	}

	supportedFormats := map[string]bool{
		"png":  true,
		"jpeg": true,
		"jpg":  true,
		"gif":  true,
		"webp": true,
		"bmp":  true,
	}

	if !supportedFormats[req.TargetFormat] {
		return ConvertResponse{}, ErrUnsupportedFormat
	}

	// Generate mock converted data
	mockData := generateMockImageData(100, 100, req.TargetFormat)

	return ConvertResponse{
		Data:   base64.StdEncoding.EncodeToString(mockData),
		Format: req.TargetFormat,
		Size:   int64(len(mockData)),
	}, nil
}

func (p *MockProvider) defaultAnalyze(_ context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	if req.Source == "" {
		return AnalyzeResponse{}, ErrInvalidInput
	}

	resp := AnalyzeResponse{
		Format:     "png",
		Width:      800,
		Height:     600,
		Size:       125000,
		ColorSpace: "sRGB",
		HasAlpha:   true,
	}

	if req.IncludeMetadata {
		resp.Metadata = map[string]interface{}{
			"created":     "2024-01-15T10:30:00Z",
			"software":    "Mock Image Provider",
			"compression": "lossless",
		}
	}

	// Check requested features
	featureSet := make(map[string]bool)
	for _, f := range req.Features {
		featureSet[f] = true
	}

	// Default to all features if none specified
	if len(featureSet) == 0 {
		featureSet["colors"] = true
		featureSet["labels"] = true
	}

	if featureSet["colors"] {
		resp.DominantColors = []Color{
			{Hex: "#3498db", RGB: [3]int{52, 152, 219}, Percentage: 35.5},
			{Hex: "#2ecc71", RGB: [3]int{46, 204, 113}, Percentage: 28.2},
			{Hex: "#e74c3c", RGB: [3]int{231, 76, 60}, Percentage: 15.8},
		}
	}

	if featureSet["labels"] {
		resp.Labels = []Label{
			{Name: "landscape", Confidence: 0.95},
			{Name: "nature", Confidence: 0.88},
			{Name: "outdoor", Confidence: 0.82},
		}
	}

	if featureSet["text"] || featureSet["ocr"] {
		resp.Text = []TextRegion{
			{
				Text:       "Sample Text",
				Confidence: 0.92,
				BoundingBox: BoundingBox{
					X: 100, Y: 50, Width: 200, Height: 30,
				},
			},
		}
	}

	if featureSet["faces"] {
		resp.Faces = []Face{
			{
				Confidence: 0.97,
				BoundingBox: BoundingBox{
					X: 200, Y: 150, Width: 100, Height: 120,
				},
				Landmarks: map[string]Point{
					"left_eye":  {X: 230, Y: 190},
					"right_eye": {X: 270, Y: 190},
					"nose":      {X: 250, Y: 220},
					"mouth":     {X: 250, Y: 250},
				},
			},
		}
	}

	return resp, nil
}

// generateMockImageData creates mock image data for testing.
func generateMockImageData(width, height int, format string) []byte {
	// Generate deterministic mock data based on dimensions and format
	size := width * height / 10 // Simplified size calculation
	if size < 100 {
		size = 100
	}

	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i + width + height) % 256)
	}

	// Add format-specific header bytes for realism
	switch format {
	case "png":
		// PNG signature
		copy(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	case "jpeg", "jpg":
		// JPEG signature
		copy(data, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	case "gif":
		// GIF signature
		copy(data, []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61})
	case "webp":
		// WebP signature
		copy(data, []byte{0x52, 0x49, 0x46, 0x46})
	case "bmp":
		// BMP signature
		copy(data, []byte{0x42, 0x4D})
	}

	return data
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
