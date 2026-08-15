// Package image provides sandboxed image processing tools for agent-go.
//
// This pack includes tools for image operations:
//   - image_metadata: Extract width, height, and format
//   - image_resize: Resize an image to specified dimensions
//   - image_crop: Crop an image to a specified region
//   - image_rotate: Rotate an image by 90, 180, or 270 degrees
//   - image_thumbnail: Generate a thumbnail constrained by max side
//   - image_convert: Convert between PNG and JPEG
//   - image_compress: Re-encode as JPEG with a quality setting
//   - image_watermark: Overlay a semi-transparent block watermark
//
// Supported formats: PNG, JPEG, GIF (decode). Encode supports PNG and JPEG.
// All paths are resolved relative to a configured base directory and
// rejected if they escape that sandbox.
package image

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif" // register GIF decoder with image.Decode

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the image processing tools pack.
// The baseDir parameter restricts all file operations to the given directory
// to prevent path traversal attacks. All paths provided by tool inputs are
// resolved relative to baseDir.
func Pack(baseDir string) *pack.Pack {
	return pack.NewBuilder("image").
		WithDescription("Image processing and manipulation tools").
		WithVersion("0.1.0").
		AddTools(
			imageMetadata(baseDir),
			imageResize(baseDir),
			imageCrop(baseDir),
			imageRotate(baseDir),
			imageThumbnail(baseDir),
			imageConvert(baseDir),
			imageCompress(baseDir),
			imageWatermark(baseDir),
		).
		AllowInState(agent.StateExplore, "image_metadata").
		AllowInState(agent.StateAct,
			"image_resize", "image_crop", "image_convert", "image_compress",
			"image_rotate", "image_thumbnail", "image_metadata", "image_watermark").
		AllowInState(agent.StateValidate, "image_metadata").
		Build()
}

// --- Path security ---

func safePath(baseDir, userPath string) (string, error) {
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("path traversal attempt: %s", userPath)
	}
	fullPath := filepath.Join(baseDir, filepath.Clean(userPath))
	if !isSubPath(baseDir, fullPath) {
		return "", fmt.Errorf("path traversal attempt: %s", userPath)
	}
	return fullPath, nil
}

func isSubPath(base, path string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
}

// --- Shared helpers ---

func decodeImage(path string) (image.Image, string, error) {
	// #nosec G304 -- path is sanitized via safePath by callers
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}
	return img, format, nil
}

func encodeImage(path, format string, img image.Image, quality int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// #nosec G304 -- path is sanitized via safePath by callers
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	switch strings.ToLower(format) {
	case "png":
		if err := png.Encode(f, img); err != nil {
			return fmt.Errorf("failed to encode png: %w", err)
		}
	case "jpeg", "jpg":
		q := quality
		if q <= 0 {
			q = 90
		}
		if q > 100 {
			q = 100
		}
		if err := jpeg.Encode(f, img, &jpeg.Options{Quality: q}); err != nil {
			return fmt.Errorf("failed to encode jpeg: %w", err)
		}
	default:
		return fmt.Errorf("unsupported output format: %s (use png or jpeg)", format)
	}
	return nil
}

func formatFromPath(path, fallback string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".gif":
		// GIF encode not supported; default to png for mutated outputs.
		if fallback != "" {
			return fallback
		}
		return "png"
	default:
		if fallback != "" {
			return fallback
		}
		return "png"
	}
}

// resizeNearest scales src to width×height using nearest-neighbor sampling.
func resizeNearest(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := src.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	if sw == 0 || sh == 0 || width == 0 || height == 0 {
		return dst
	}
	for y := 0; y < height; y++ {
		sy := bounds.Min.Y + y*sh/height
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*sw/width
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func rotateImage(src image.Image, degrees int) (image.Image, error) {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	switch degrees {
	case 0:
		return src, nil
	case 90:
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				nx := bounds.Max.Y - 1 - y
				ny := x - bounds.Min.X
				dst.Set(nx, ny, src.At(x, y))
			}
		}
		return dst, nil
	case 180:
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				nx := bounds.Max.X - 1 - x
				ny := bounds.Max.Y - 1 - y
				dst.Set(nx-bounds.Min.X, ny-bounds.Min.Y, src.At(x, y))
			}
		}
		return dst, nil
	case 270:
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				nx := y - bounds.Min.Y
				ny := bounds.Max.X - 1 - x
				dst.Set(nx, ny, src.At(x, y))
			}
		}
		return dst, nil
	default:
		return nil, fmt.Errorf("unsupported rotation degrees: %d (use 90, 180, or 270)", degrees)
	}
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func jsonResult(v any) (tool.Result, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to marshal output: %w", err)
	}
	return tool.Result{Output: b}, nil
}

// --- Input/Output types ---

type metadataInput struct {
	Path string `json:"path"`
}

type metadataOutput struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Path   string `json:"path"`
}

type resizeInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type cropInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type rotateInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	Degrees    int    `json:"degrees"`
}

type thumbnailInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	MaxSide    int    `json:"max_side"`
}

type convertInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
}

type compressInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	Quality    int    `json:"quality"`
}

type watermarkInput struct {
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	// Opacity is 0–100 for the block watermark (default 40).
	Opacity int `json:"opacity,omitempty"`
	// Size is the watermark block side as a fraction of the shorter image side
	// in percent (default 20).
	Size int `json:"size,omitempty"`
}

type transformOutput struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

// --- Tools ---

func imageMetadata(baseDir string) tool.Tool {
	return tool.NewBuilder("image_metadata").
		WithDescription("Extract width, height, and format from an image").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path": json.RawMessage(`{"type":"string","description":"Path to the image (relative to base directory)"}`),
		}, []string{"path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in metadataInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(fullPath)
			if err != nil {
				return tool.Result{}, err
			}
			b := img.Bounds()
			return jsonResult(metadataOutput{
				Width:  b.Dx(),
				Height: b.Dy(),
				Format: format,
				Path:   in.Path,
			})
		}).
		MustBuild()
}

func imageResize(baseDir string) tool.Tool {
	return tool.NewBuilder("image_resize").
		WithDescription("Resize an image to specified dimensions using nearest-neighbor sampling").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"width":       json.RawMessage(`{"type":"integer","description":"Target width in pixels","minimum":1}`),
			"height":      json.RawMessage(`{"type":"integer","description":"Target height in pixels","minimum":1}`),
		}, []string{"path", "output_path", "width", "height"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in resizeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			if in.Width <= 0 || in.Height <= 0 {
				return tool.Result{}, fmt.Errorf("width and height must be positive")
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			outFmt := formatFromPath(in.OutputPath, format)
			resized := resizeNearest(img, in.Width, in.Height)
			if err := encodeImage(dstPath, outFmt, resized, 90); err != nil {
				return tool.Result{}, err
			}
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  in.Width,
				Height: in.Height,
				Format: outFmt,
			})
		}).
		MustBuild()
}

func imageCrop(baseDir string) tool.Tool {
	return tool.NewBuilder("image_crop").
		WithDescription("Crop an image to a specified rectangular region").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"x":           json.RawMessage(`{"type":"integer","description":"Left offset in pixels","minimum":0}`),
			"y":           json.RawMessage(`{"type":"integer","description":"Top offset in pixels","minimum":0}`),
			"width":       json.RawMessage(`{"type":"integer","description":"Crop width in pixels","minimum":1}`),
			"height":      json.RawMessage(`{"type":"integer","description":"Crop height in pixels","minimum":1}`),
		}, []string{"path", "output_path", "x", "y", "width", "height"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in cropInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			if in.X < 0 || in.Y < 0 || in.Width <= 0 || in.Height <= 0 {
				return tool.Result{}, fmt.Errorf("invalid crop rectangle")
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			bounds := img.Bounds()
			cropRect := image.Rect(
				bounds.Min.X+in.X,
				bounds.Min.Y+in.Y,
				bounds.Min.X+in.X+in.Width,
				bounds.Min.Y+in.Y+in.Height,
			)
			if !cropRect.In(bounds) {
				return tool.Result{}, fmt.Errorf("crop region exceeds image bounds (%dx%d)", bounds.Dx(), bounds.Dy())
			}
			dst := image.NewRGBA(image.Rect(0, 0, in.Width, in.Height))
			draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)
			outFmt := formatFromPath(in.OutputPath, format)
			if err := encodeImage(dstPath, outFmt, dst, 90); err != nil {
				return tool.Result{}, err
			}
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  in.Width,
				Height: in.Height,
				Format: outFmt,
			})
		}).
		MustBuild()
}

func imageRotate(baseDir string) tool.Tool {
	return tool.NewBuilder("image_rotate").
		WithDescription("Rotate an image by 90, 180, or 270 degrees clockwise").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"degrees":     json.RawMessage(`{"type":"integer","description":"Rotation in degrees (90, 180, or 270)","enum":[90,180,270]}`),
		}, []string{"path", "output_path", "degrees"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in rotateInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			rotated, err := rotateImage(img, in.Degrees)
			if err != nil {
				return tool.Result{}, err
			}
			outFmt := formatFromPath(in.OutputPath, format)
			if err := encodeImage(dstPath, outFmt, rotated, 90); err != nil {
				return tool.Result{}, err
			}
			b := rotated.Bounds()
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  b.Dx(),
				Height: b.Dy(),
				Format: outFmt,
			})
		}).
		MustBuild()
}

func imageThumbnail(baseDir string) tool.Tool {
	return tool.NewBuilder("image_thumbnail").
		WithDescription("Generate a thumbnail constrained by a maximum side length, preserving aspect ratio").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"max_side":    json.RawMessage(`{"type":"integer","description":"Maximum length of the longer side in pixels","minimum":1}`),
		}, []string{"path", "output_path", "max_side"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in thumbnailInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			if in.MaxSide <= 0 {
				return tool.Result{}, fmt.Errorf("max_side must be positive")
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			b := img.Bounds()
			w, h := b.Dx(), b.Dy()
			tw, th := w, h
			if w >= h {
				if w > in.MaxSide {
					tw = in.MaxSide
					th = h * in.MaxSide / w
					if th < 1 {
						th = 1
					}
				}
			} else if h > in.MaxSide {
				th = in.MaxSide
				tw = w * in.MaxSide / h
				if tw < 1 {
					tw = 1
				}
			}
			thumb := resizeNearest(img, tw, th)
			outFmt := formatFromPath(in.OutputPath, format)
			if err := encodeImage(dstPath, outFmt, thumb, 90); err != nil {
				return tool.Result{}, err
			}
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  tw,
				Height: th,
				Format: outFmt,
			})
		}).
		MustBuild()
}

func imageConvert(baseDir string) tool.Tool {
	return tool.NewBuilder("image_convert").
		WithDescription("Convert an image to PNG or JPEG format").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"format":      json.RawMessage(`{"type":"string","description":"Output format","enum":["png","jpeg"]}`),
		}, []string{"path", "output_path", "format"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in convertInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			fmtName := strings.ToLower(in.Format)
			if fmtName == "jpg" {
				fmtName = "jpeg"
			}
			if fmtName != "png" && fmtName != "jpeg" {
				return tool.Result{}, fmt.Errorf("unsupported format: %s (use png or jpeg)", in.Format)
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, _, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			if err := encodeImage(dstPath, fmtName, img, 90); err != nil {
				return tool.Result{}, err
			}
			b := img.Bounds()
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  b.Dx(),
				Height: b.Dy(),
				Format: fmtName,
			})
		}).
		MustBuild()
}

func imageCompress(baseDir string) tool.Tool {
	return tool.NewBuilder("image_compress").
		WithDescription("Compress an image by re-encoding as JPEG with a quality setting").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"quality":     json.RawMessage(`{"type":"integer","description":"JPEG quality from 1 to 100","minimum":1,"maximum":100}`),
		}, []string{"path", "output_path", "quality"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in compressInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			if in.Quality < 1 || in.Quality > 100 {
				return tool.Result{}, fmt.Errorf("quality must be between 1 and 100")
			}
			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, _, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}
			if err := encodeImage(dstPath, "jpeg", img, in.Quality); err != nil {
				return tool.Result{}, err
			}
			b := img.Bounds()
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  b.Dx(),
				Height: b.Dy(),
				Format: "jpeg",
			})
		}).
		MustBuild()
}

func imageWatermark(baseDir string) tool.Tool {
	return tool.NewBuilder("image_watermark").
		WithDescription("Add a semi-transparent block watermark in the bottom-right corner (placeholder for text watermarks)").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"path":        json.RawMessage(`{"type":"string","description":"Source image path (relative to base directory)"}`),
			"output_path": json.RawMessage(`{"type":"string","description":"Destination image path (relative to base directory)"}`),
			"opacity":     json.RawMessage(`{"type":"integer","description":"Block opacity 0-100 (default 40)","minimum":0,"maximum":100}`),
			"size":        json.RawMessage(`{"type":"integer","description":"Block size as percent of shorter side (default 20)","minimum":1,"maximum":50}`),
		}, []string{"path", "output_path"})).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in watermarkInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}
			opacity := in.Opacity
			if opacity == 0 {
				opacity = 40
			}
			if opacity < 0 || opacity > 100 {
				return tool.Result{}, fmt.Errorf("opacity must be between 0 and 100")
			}
			sizePct := in.Size
			if sizePct == 0 {
				sizePct = 20
			}
			if sizePct < 1 || sizePct > 50 {
				return tool.Result{}, fmt.Errorf("size must be between 1 and 50")
			}

			srcPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			dstPath, err := safePath(baseDir, in.OutputPath)
			if err != nil {
				return tool.Result{}, err
			}
			img, format, err := decodeImage(srcPath)
			if err != nil {
				return tool.Result{}, err
			}

			dst := toRGBA(img)
			b := dst.Bounds()
			short := b.Dx()
			if b.Dy() < short {
				short = b.Dy()
			}
			block := short * sizePct / 100
			if block < 1 {
				block = 1
			}
			margin := block / 4
			if margin < 1 {
				margin = 1
			}
			rect := image.Rect(
				b.Max.X-margin-block,
				b.Max.Y-margin-block,
				b.Max.X-margin,
				b.Max.Y-margin,
			)
			alpha := uint8(opacity * 255 / 100)
			overlay := image.NewUniform(color.NRGBA{R: 0, G: 0, B: 0, A: alpha})
			draw.Draw(dst, rect, overlay, image.Point{}, draw.Over)

			outFmt := formatFromPath(in.OutputPath, format)
			if err := encodeImage(dstPath, outFmt, dst, 90); err != nil {
				return tool.Result{}, err
			}
			return jsonResult(transformOutput{
				Path:   in.OutputPath,
				Width:  b.Dx(),
				Height: b.Dy(),
				Format: outFmt,
			})
		}).
		MustBuild()
}
