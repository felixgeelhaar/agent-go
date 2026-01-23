package pdf

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the PDF pack.
type PackConfig struct {
	// Provider is the PDF provider to use.
	Provider Provider

	// DefaultPageSize is the default page size.
	DefaultPageSize string

	// DefaultOrientation is the default page orientation.
	DefaultOrientation string
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		DefaultPageSize:    "A4",
		DefaultOrientation: "portrait",
	}
}

// New creates a new PDF pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.DefaultPageSize == "" {
		cfg.DefaultPageSize = "A4"
	}
	if cfg.DefaultOrientation == "" {
		cfg.DefaultOrientation = "portrait"
	}

	return pack.NewBuilder("pdf").
		WithDescription("Tools for PDF generation, parsing, and merging").
		WithVersion("1.0.0").
		AddTools(
			generateTool(cfg),
			parseTool(cfg),
			mergeTool(cfg),
		).
		AllowInState(agent.StateExplore, "pdf_parse").
		AllowInState(agent.StateAct, "pdf_generate", "pdf_parse", "pdf_merge").
		AllowInState(agent.StateValidate, "pdf_parse").
		Build()
}

// generateTool creates the pdf_generate tool.
func generateTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("pdf_generate").
		WithDescription("Generate a PDF from content").
		WithTags("pdf", "document", "generation").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Content     string                 `json:"content"`
				Format      string                 `json:"format"`
				Template    string                 `json:"template"`
				Variables   map[string]interface{} `json:"variables"`
				PageSize    string                 `json:"page_size"`
				Orientation string                 `json:"orientation"`
				Margins     Margins                `json:"margins"`
				Header      string                 `json:"header"`
				Footer      string                 `json:"footer"`
				Watermark   string                 `json:"watermark"`
				Metadata    Metadata               `json:"metadata"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Content == "" && req.Template == "" {
				return tool.Result{}, ErrInvalidInput
			}

			pageSize := req.PageSize
			if pageSize == "" {
				pageSize = cfg.DefaultPageSize
			}

			orientation := req.Orientation
			if orientation == "" {
				orientation = cfg.DefaultOrientation
			}

			resp, err := cfg.Provider.Generate(ctx, GenerateRequest{
				Content:   req.Content,
				Format:    req.Format,
				Template:  req.Template,
				Variables: req.Variables,
				Options: GenerateOptions{
					PageSize:    pageSize,
					Orientation: orientation,
					Margins:     req.Margins,
					Header:      req.Header,
					Footer:      req.Footer,
					Watermark:   req.Watermark,
					Metadata:    req.Metadata,
				},
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseTool creates the pdf_parse tool.
func parseTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("pdf_parse").
		WithDescription("Extract content from a PDF").
		ReadOnly().
		WithTags("pdf", "document", "parsing").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Content        string `json:"content"`
				ExtractImages  bool   `json:"extract_images"`
				ExtractTables  bool   `json:"extract_tables"`
				PageRange      string `json:"page_range"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Content == "" {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Parse(ctx, ParseRequest{
				Content:        req.Content,
				ExtractImages:  req.ExtractImages,
				ExtractTables:  req.ExtractTables,
				PageRange:      req.PageRange,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// mergeTool creates the pdf_merge tool.
func mergeTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("pdf_merge").
		WithDescription("Merge multiple PDFs into one").
		WithTags("pdf", "document", "merge").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Documents      []string `json:"documents"`
				Bookmarks      bool     `json:"bookmarks"`
				BookmarkTitles []string `json:"bookmark_titles"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if len(req.Documents) < 2 {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Merge(ctx, MergeRequest{
				Documents:      req.Documents,
				Bookmarks:      req.Bookmarks,
				BookmarkTitles: req.BookmarkTitles,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
