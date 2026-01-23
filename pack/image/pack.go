package image

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the image processing pack.
type PackConfig struct {
	// Provider is the image processing provider to use.
	Provider Provider

	// DefaultQuality is the default output quality (1-100).
	DefaultQuality int

	// MaxImageSize is the maximum input image size in bytes.
	MaxImageSize int64
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		DefaultQuality: 85,
		MaxImageSize:   50 * 1024 * 1024, // 50MB
	}
}

// New creates a new image processing pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.DefaultQuality == 0 {
		cfg.DefaultQuality = 85
	}
	if cfg.MaxImageSize == 0 {
		cfg.MaxImageSize = 50 * 1024 * 1024
	}

	return pack.NewBuilder("image").
		WithDescription("Tools for image processing and analysis").
		WithVersion("1.0.0").
		AddTools(
			resizeTool(cfg),
			convertTool(cfg),
			analyzeTool(cfg),
		).
		AllowInState(agent.StateExplore, "image_analyze").
		AllowInState(agent.StateAct, "image_resize", "image_convert", "image_analyze").
		Build()
}

// resizeTool creates the image_resize tool.
func resizeTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("image_resize").
		WithDescription("Resize an image to specified dimensions").
		WithTags("image", "transform", "resize").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Source       string `json:"source"`
				SourceType   string `json:"source_type"`
				Width        int    `json:"width"`
				Height       int    `json:"height"`
				Mode         string `json:"mode"`
				Quality      int    `json:"quality"`
				OutputFormat string `json:"output_format"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Source == "" || req.Width <= 0 || req.Height <= 0 {
				return tool.Result{}, ErrInvalidInput
			}

			// Apply defaults
			sourceType := req.SourceType
			if sourceType == "" {
				sourceType = "base64"
			}

			mode := req.Mode
			if mode == "" {
				mode = "fit"
			}

			quality := req.Quality
			if quality == 0 {
				quality = cfg.DefaultQuality
			}

			resp, err := cfg.Provider.Resize(ctx, ResizeRequest{
				Source:       req.Source,
				SourceType:   sourceType,
				Width:        req.Width,
				Height:       req.Height,
				Mode:         mode,
				Quality:      quality,
				OutputFormat: req.OutputFormat,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// convertTool creates the image_convert tool.
func convertTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("image_convert").
		WithDescription("Convert an image to a different format").
		WithTags("image", "transform", "convert").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Source        string `json:"source"`
				SourceType    string `json:"source_type"`
				TargetFormat  string `json:"target_format"`
				Quality       int    `json:"quality"`
				StripMetadata bool   `json:"strip_metadata"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Source == "" || req.TargetFormat == "" {
				return tool.Result{}, ErrInvalidInput
			}

			// Apply defaults
			sourceType := req.SourceType
			if sourceType == "" {
				sourceType = "base64"
			}

			quality := req.Quality
			if quality == 0 {
				quality = cfg.DefaultQuality
			}

			resp, err := cfg.Provider.Convert(ctx, ConvertRequest{
				Source:        req.Source,
				SourceType:    sourceType,
				TargetFormat:  req.TargetFormat,
				Quality:       quality,
				StripMetadata: req.StripMetadata,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// analyzeTool creates the image_analyze tool.
func analyzeTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("image_analyze").
		WithDescription("Analyze an image and extract metadata and features").
		ReadOnly().
		Cacheable().
		WithTags("image", "analysis", "ai").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Source          string   `json:"source"`
				SourceType      string   `json:"source_type"`
				Features        []string `json:"features"`
				IncludeMetadata bool     `json:"include_metadata"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Source == "" {
				return tool.Result{}, ErrInvalidInput
			}

			// Apply defaults
			sourceType := req.SourceType
			if sourceType == "" {
				sourceType = "base64"
			}

			resp, err := cfg.Provider.Analyze(ctx, AnalyzeRequest{
				Source:          req.Source,
				SourceType:      sourceType,
				Features:        req.Features,
				IncludeMetadata: req.IncludeMetadata,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output: output,
				Cached: true,
			}, nil
		}).
		MustBuild()
}
