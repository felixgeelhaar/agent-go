// Package pdf provides tools for generating, parsing, and merging PDFs.
package pdf

import (
	"context"
	"errors"
)

// Common errors for PDF operations.
var (
	ErrProviderNotConfigured = errors.New("pdf provider not configured")
	ErrInvalidInput          = errors.New("invalid input")
	ErrGenerationFailed      = errors.New("pdf generation failed")
	ErrParseFailed           = errors.New("pdf parse failed")
	ErrMergeFailed           = errors.New("pdf merge failed")
)

// Provider defines the interface for PDF providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Generate generates a PDF from content.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)

	// Parse extracts content from a PDF.
	Parse(ctx context.Context, req ParseRequest) (ParseResponse, error)

	// Merge combines multiple PDFs into one.
	Merge(ctx context.Context, req MergeRequest) (MergeResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// GenerateRequest represents a request to generate a PDF.
type GenerateRequest struct {
	// Content is the source content.
	Content string `json:"content"`

	// Format is the source format (html, markdown, text).
	Format string `json:"format"`

	// Template is an optional template name.
	Template string `json:"template,omitempty"`

	// Variables are template variables.
	Variables map[string]interface{} `json:"variables,omitempty"`

	// Options are generation options.
	Options GenerateOptions `json:"options,omitempty"`
}

// GenerateOptions contains PDF generation options.
type GenerateOptions struct {
	// PageSize is the page size (A4, Letter, Legal).
	PageSize string `json:"page_size,omitempty"`

	// Orientation is page orientation (portrait, landscape).
	Orientation string `json:"orientation,omitempty"`

	// Margins are page margins in points.
	Margins Margins `json:"margins,omitempty"`

	// Header is the page header content.
	Header string `json:"header,omitempty"`

	// Footer is the page footer content.
	Footer string `json:"footer,omitempty"`

	// Watermark is the watermark text.
	Watermark string `json:"watermark,omitempty"`

	// Metadata is PDF metadata.
	Metadata Metadata `json:"metadata,omitempty"`
}

// Margins represents page margins.
type Margins struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// Metadata represents PDF metadata.
type Metadata struct {
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Keywords string `json:"keywords,omitempty"`
	Creator  string `json:"creator,omitempty"`
}

// GenerateResponse represents the result of PDF generation.
type GenerateResponse struct {
	// Content is the generated PDF content (base64 encoded).
	Content string `json:"content"`

	// Size is the PDF size in bytes.
	Size int64 `json:"size"`

	// PageCount is the number of pages.
	PageCount int `json:"page_count"`

	// Success indicates if generation succeeded.
	Success bool `json:"success"`
}

// ParseRequest represents a request to parse a PDF.
type ParseRequest struct {
	// Content is the PDF content (base64 encoded).
	Content string `json:"content"`

	// ExtractImages indicates whether to extract images.
	ExtractImages bool `json:"extract_images,omitempty"`

	// ExtractTables indicates whether to extract tables.
	ExtractTables bool `json:"extract_tables,omitempty"`

	// PageRange specifies pages to parse (e.g., "1-5", "1,3,5").
	PageRange string `json:"page_range,omitempty"`
}

// ParseResponse represents a parsed PDF.
type ParseResponse struct {
	// Text is the extracted text content.
	Text string `json:"text"`

	// Pages are the individual page contents.
	Pages []PageContent `json:"pages"`

	// Metadata is the PDF metadata.
	Metadata Metadata `json:"metadata,omitempty"`

	// Images are extracted images.
	Images []ExtractedImage `json:"images,omitempty"`

	// Tables are extracted tables.
	Tables []ExtractedTable `json:"tables,omitempty"`

	// PageCount is the total number of pages.
	PageCount int `json:"page_count"`
}

// PageContent represents content from a single page.
type PageContent struct {
	// Number is the page number (1-indexed).
	Number int `json:"number"`

	// Text is the page text content.
	Text string `json:"text"`

	// Width is the page width in points.
	Width float64 `json:"width"`

	// Height is the page height in points.
	Height float64 `json:"height"`
}

// ExtractedImage represents an extracted image.
type ExtractedImage struct {
	// Page is the source page number.
	Page int `json:"page"`

	// Content is the image content (base64 encoded).
	Content string `json:"content"`

	// Format is the image format.
	Format string `json:"format"`

	// Width is the image width.
	Width int `json:"width"`

	// Height is the image height.
	Height int `json:"height"`
}

// ExtractedTable represents an extracted table.
type ExtractedTable struct {
	// Page is the source page number.
	Page int `json:"page"`

	// Headers are the table headers.
	Headers []string `json:"headers,omitempty"`

	// Rows are the table rows.
	Rows [][]string `json:"rows"`
}

// MergeRequest represents a request to merge PDFs.
type MergeRequest struct {
	// Documents are the PDFs to merge (base64 encoded).
	Documents []string `json:"documents"`

	// Bookmarks indicates whether to create bookmarks.
	Bookmarks bool `json:"bookmarks,omitempty"`

	// BookmarkTitles are titles for bookmarks.
	BookmarkTitles []string `json:"bookmark_titles,omitempty"`
}

// MergeResponse represents the result of merging PDFs.
type MergeResponse struct {
	// Content is the merged PDF content (base64 encoded).
	Content string `json:"content"`

	// Size is the merged PDF size in bytes.
	Size int64 `json:"size"`

	// PageCount is the total number of pages.
	PageCount int `json:"page_count"`

	// Success indicates if merging succeeded.
	Success bool `json:"success"`
}
