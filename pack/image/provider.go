// Package image provides tools for image processing operations.
package image

import (
	"context"
	"errors"
)

// Common errors for image operations.
var (
	ErrProviderNotConfigured = errors.New("image provider not configured")
	ErrInvalidInput          = errors.New("invalid input")
	ErrUnsupportedFormat     = errors.New("unsupported image format")
	ErrImageTooLarge         = errors.New("image too large")
	ErrProviderUnavailable   = errors.New("provider unavailable")
)

// Provider defines the interface for image processing providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Resize resizes an image to the specified dimensions.
	Resize(ctx context.Context, req ResizeRequest) (ResizeResponse, error)

	// Convert converts an image to a different format.
	Convert(ctx context.Context, req ConvertRequest) (ConvertResponse, error)

	// Analyze analyzes an image and returns metadata and features.
	Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// ResizeRequest represents a request to resize an image.
type ResizeRequest struct {
	// Source is the input image data (base64 encoded) or URL.
	Source string `json:"source"`

	// SourceType indicates if source is "base64" or "url".
	SourceType string `json:"source_type"`

	// Width is the target width in pixels.
	Width int `json:"width"`

	// Height is the target height in pixels.
	Height int `json:"height"`

	// Mode specifies resize behavior: "fit", "fill", "stretch".
	Mode string `json:"mode,omitempty"`

	// Quality is the output quality (1-100), applicable for lossy formats.
	Quality int `json:"quality,omitempty"`

	// OutputFormat specifies the output format (e.g., "png", "jpeg").
	OutputFormat string `json:"output_format,omitempty"`
}

// ResizeResponse represents the result of a resize operation.
type ResizeResponse struct {
	// Data is the resized image (base64 encoded).
	Data string `json:"data"`

	// Format is the output format.
	Format string `json:"format"`

	// Width is the actual output width.
	Width int `json:"width"`

	// Height is the actual output height.
	Height int `json:"height"`

	// Size is the output size in bytes.
	Size int64 `json:"size"`
}

// ConvertRequest represents a request to convert an image format.
type ConvertRequest struct {
	// Source is the input image data (base64 encoded) or URL.
	Source string `json:"source"`

	// SourceType indicates if source is "base64" or "url".
	SourceType string `json:"source_type"`

	// TargetFormat is the desired output format.
	TargetFormat string `json:"target_format"`

	// Quality is the output quality (1-100), applicable for lossy formats.
	Quality int `json:"quality,omitempty"`

	// StripMetadata removes EXIF and other metadata if true.
	StripMetadata bool `json:"strip_metadata,omitempty"`
}

// ConvertResponse represents the result of a convert operation.
type ConvertResponse struct {
	// Data is the converted image (base64 encoded).
	Data string `json:"data"`

	// Format is the output format.
	Format string `json:"format"`

	// Size is the output size in bytes.
	Size int64 `json:"size"`
}

// AnalyzeRequest represents a request to analyze an image.
type AnalyzeRequest struct {
	// Source is the input image data (base64 encoded) or URL.
	Source string `json:"source"`

	// SourceType indicates if source is "base64" or "url".
	SourceType string `json:"source_type"`

	// Features specifies which analysis features to run.
	Features []string `json:"features,omitempty"`

	// IncludeMetadata includes EXIF and other metadata.
	IncludeMetadata bool `json:"include_metadata,omitempty"`
}

// AnalyzeResponse represents the result of an image analysis.
type AnalyzeResponse struct {
	// Format is the detected image format.
	Format string `json:"format"`

	// Width is the image width in pixels.
	Width int `json:"width"`

	// Height is the image height in pixels.
	Height int `json:"height"`

	// Size is the file size in bytes.
	Size int64 `json:"size"`

	// ColorSpace is the detected color space.
	ColorSpace string `json:"color_space,omitempty"`

	// HasAlpha indicates if the image has an alpha channel.
	HasAlpha bool `json:"has_alpha,omitempty"`

	// Metadata contains EXIF and other metadata if requested.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// DominantColors are the dominant colors in the image.
	DominantColors []Color `json:"dominant_colors,omitempty"`

	// Labels are detected objects/concepts in the image.
	Labels []Label `json:"labels,omitempty"`

	// Text contains detected text (OCR results).
	Text []TextRegion `json:"text,omitempty"`

	// Faces contains detected face information.
	Faces []Face `json:"faces,omitempty"`
}

// Color represents a color with its frequency.
type Color struct {
	// Hex is the color in hexadecimal format.
	Hex string `json:"hex"`

	// RGB is the color as RGB values.
	RGB [3]int `json:"rgb"`

	// Percentage is the color frequency in the image.
	Percentage float64 `json:"percentage"`
}

// Label represents a detected label/object.
type Label struct {
	// Name is the label name.
	Name string `json:"name"`

	// Confidence is the confidence score (0-1).
	Confidence float64 `json:"confidence"`

	// BoundingBox is the optional region for the label.
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
}

// TextRegion represents detected text in an image.
type TextRegion struct {
	// Text is the detected text content.
	Text string `json:"text"`

	// Confidence is the confidence score (0-1).
	Confidence float64 `json:"confidence"`

	// BoundingBox is the region containing the text.
	BoundingBox BoundingBox `json:"bounding_box"`
}

// Face represents a detected face.
type Face struct {
	// Confidence is the detection confidence (0-1).
	Confidence float64 `json:"confidence"`

	// BoundingBox is the face region.
	BoundingBox BoundingBox `json:"bounding_box"`

	// Landmarks are facial landmarks if detected.
	Landmarks map[string]Point `json:"landmarks,omitempty"`
}

// BoundingBox represents a rectangular region.
type BoundingBox struct {
	// X is the left coordinate.
	X int `json:"x"`

	// Y is the top coordinate.
	Y int `json:"y"`

	// Width is the box width.
	Width int `json:"width"`

	// Height is the box height.
	Height int `json:"height"`
}

// Point represents a 2D coordinate.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
