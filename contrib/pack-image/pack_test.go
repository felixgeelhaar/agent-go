package image

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: 128,
				A: 255,
			})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func getTool(t *testing.T, baseDir, name string) tool.Tool {
	t.Helper()
	p := Pack(baseDir)
	tt, ok := p.GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found in pack", name)
	}
	return tt
}

func execTool(t *testing.T, tt tool.Tool, input any) json.RawMessage {
	t.Helper()
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	result, err := tt.Execute(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("tool %q execution failed: %v", tt.Name(), err)
	}
	return result.Output
}

func execToolErr(t *testing.T, tt tool.Tool, input any) error {
	t.Helper()
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	_, err = tt.Execute(context.Background(), inputBytes)
	return err
}

func TestPack_RegistersTools(t *testing.T) {
	p := Pack(t.TempDir())
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Metadata["status"] == "stub" {
		t.Fatal("expected no stub metadata")
	}
	if strings.Contains(p.Description, "STUB") {
		t.Errorf("description should not mention STUB: %q", p.Description)
	}
	if p.Name != "image" {
		t.Errorf("expected pack name %q, got %q", "image", p.Name)
	}
	if got := len(p.Tools); got != 8 {
		t.Errorf("expected 8 tools, got %d", got)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack(t.TempDir())
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
		if tt.InputSchema().IsEmpty() {
			t.Errorf("tool %q has empty input schema", tt.Name())
		}
	}
}

func TestAnnotationsPreserved(t *testing.T) {
	p := Pack(t.TempDir())
	checks := map[string]func(tool.Annotations) bool{
		"image_metadata":  func(a tool.Annotations) bool { return a.ReadOnly && a.Cacheable },
		"image_resize":    func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_crop":      func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_rotate":    func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_thumbnail": func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_convert":   func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_compress":  func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"image_watermark": func(a tool.Annotations) bool { return a.RiskLevel == tool.RiskLow },
	}
	for name, check := range checks {
		tt, ok := p.GetTool(name)
		if !ok {
			t.Fatalf("tool %q not found", name)
		}
		if !check(tt.Annotations()) {
			t.Errorf("tool %q annotations not preserved: %+v", name, tt.Annotations())
		}
	}
}

func TestImageMetadata(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 40, 30)

	out := execTool(t, getTool(t, baseDir, "image_metadata"), map[string]any{"path": "src.png"})
	var meta metadataOutput
	if err := json.Unmarshal(out, &meta); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if meta.Width != 40 || meta.Height != 30 {
		t.Errorf("expected 40x30, got %dx%d", meta.Width, meta.Height)
	}
	if meta.Format != "png" {
		t.Errorf("expected format png, got %q", meta.Format)
	}
}

func TestImageResize(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 40, 30)

	out := execTool(t, getTool(t, baseDir, "image_resize"), map[string]any{
		"path": "src.png", "output_path": "out/resized.png", "width": 20, "height": 10,
	})
	var result transformOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Width != 20 || result.Height != 10 {
		t.Errorf("expected 20x10, got %dx%d", result.Width, result.Height)
	}

	f, err := os.Open(filepath.Join(baseDir, "out", "resized.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Errorf("decoded size %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestImageCrop(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 40, 30)

	out := execTool(t, getTool(t, baseDir, "image_crop"), map[string]any{
		"path": "src.png", "output_path": "cropped.png",
		"x": 5, "y": 5, "width": 10, "height": 8,
	})
	var result transformOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Width != 10 || result.Height != 8 {
		t.Errorf("expected 10x8, got %dx%d", result.Width, result.Height)
	}

	err := execToolErr(t, getTool(t, baseDir, "image_crop"), map[string]any{
		"path": "src.png", "output_path": "bad.png",
		"x": 35, "y": 0, "width": 10, "height": 8,
	})
	if err == nil {
		t.Fatal("expected crop bounds error")
	}
}

func TestImageRotate(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 40, 20)

	cases := []struct {
		degrees    int
		wantWidth  int
		wantHeight int
	}{
		{90, 20, 40},
		{180, 40, 20},
		{270, 20, 40},
	}
	for _, tc := range cases {
		name := filepath.Join("rot", "d"+itoa(tc.degrees)+".png")
		out := execTool(t, getTool(t, baseDir, "image_rotate"), map[string]any{
			"path": "src.png", "output_path": name, "degrees": tc.degrees,
		})
		var result transformOutput
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("degrees %d unmarshal: %v", tc.degrees, err)
		}
		if result.Width != tc.wantWidth || result.Height != tc.wantHeight {
			t.Errorf("degrees %d: expected %dx%d, got %dx%d",
				tc.degrees, tc.wantWidth, tc.wantHeight, result.Width, result.Height)
		}
	}

	err := execToolErr(t, getTool(t, baseDir, "image_rotate"), map[string]any{
		"path": "src.png", "output_path": "bad.png", "degrees": 45,
	})
	if err == nil {
		t.Fatal("expected unsupported degrees error")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestImageThumbnail(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "wide.png"), 100, 40)
	writeTestPNG(t, filepath.Join(baseDir, "tall.png"), 40, 100)

	out := execTool(t, getTool(t, baseDir, "image_thumbnail"), map[string]any{
		"path": "wide.png", "output_path": "thumb-wide.png", "max_side": 50,
	})
	var result transformOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Width != 50 || result.Height != 20 {
		t.Errorf("wide thumb expected 50x20, got %dx%d", result.Width, result.Height)
	}

	out = execTool(t, getTool(t, baseDir, "image_thumbnail"), map[string]any{
		"path": "tall.png", "output_path": "thumb-tall.png", "max_side": 50,
	})
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Width != 20 || result.Height != 50 {
		t.Errorf("tall thumb expected 20x50, got %dx%d", result.Width, result.Height)
	}
}

func TestImageConvertAndCompress(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 32, 32)

	out := execTool(t, getTool(t, baseDir, "image_convert"), map[string]any{
		"path": "src.png", "output_path": "converted.jpg", "format": "jpeg",
	})
	var result transformOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal convert: %v", err)
	}
	if result.Format != "jpeg" {
		t.Errorf("expected jpeg, got %q", result.Format)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "converted.jpg")); err != nil {
		t.Fatalf("converted file missing: %v", err)
	}

	out = execTool(t, getTool(t, baseDir, "image_compress"), map[string]any{
		"path": "src.png", "output_path": "compressed.jpg", "quality": 50,
	})
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal compress: %v", err)
	}
	if result.Format != "jpeg" {
		t.Errorf("expected jpeg, got %q", result.Format)
	}
	info, err := os.Stat(filepath.Join(baseDir, "compressed.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty compressed file")
	}

	err = execToolErr(t, getTool(t, baseDir, "image_convert"), map[string]any{
		"path": "src.png", "output_path": "x.webp", "format": "webp",
	})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestImageWatermark(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 50, 50)

	out := execTool(t, getTool(t, baseDir, "image_watermark"), map[string]any{
		"path": "src.png", "output_path": "marked.png", "opacity": 60, "size": 20,
	})
	var result transformOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Width != 50 || result.Height != 50 {
		t.Errorf("expected 50x50, got %dx%d", result.Width, result.Height)
	}

	f, err := os.Open(filepath.Join(baseDir, "marked.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	srcFile, err := os.Open(filepath.Join(baseDir, "src.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer srcFile.Close()
	srcImg, err := png.Decode(srcFile)
	if err != nil {
		t.Fatal(err)
	}

	// Bottom-right region should be darker than the unmodified source.
	sr, sg, sb, _ := srcImg.At(45, 45).RGBA()
	mr, mg, mb, _ := img.At(45, 45).RGBA()
	if mr >= sr && mg >= sg && mb >= sb {
		t.Errorf("expected watermark to darken corner; before=%d,%d,%d after=%d,%d,%d",
			sr>>8, sg>>8, sb>>8, mr>>8, mg>>8, mb>>8)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPNG(t, filepath.Join(baseDir, "src.png"), 8, 8)

	tools := []struct {
		name  string
		input map[string]any
	}{
		{"image_metadata", map[string]any{"path": "../../etc/passwd"}},
		{"image_resize", map[string]any{"path": "../x.png", "output_path": "o.png", "width": 1, "height": 1}},
		{"image_resize", map[string]any{"path": "src.png", "output_path": "../o.png", "width": 1, "height": 1}},
		{"image_crop", map[string]any{"path": "/etc/passwd", "output_path": "o.png", "x": 0, "y": 0, "width": 1, "height": 1}},
		{"image_rotate", map[string]any{"path": "src.png", "output_path": "../../o.png", "degrees": 90}},
		{"image_thumbnail", map[string]any{"path": "../src.png", "output_path": "o.png", "max_side": 4}},
		{"image_convert", map[string]any{"path": "../../x", "output_path": "o.png", "format": "png"}},
		{"image_compress", map[string]any{"path": "src.png", "output_path": "/tmp/o.jpg", "quality": 50}},
		{"image_watermark", map[string]any{"path": "../x.png", "output_path": "o.png"}},
	}

	for i, tc := range tools {
		t.Run(tc.name+"_"+itoa(i), func(t *testing.T) {
			err := execToolErr(t, getTool(t, baseDir, tc.name), tc.input)
			if err == nil {
				t.Error("expected error for path traversal")
			}
			if !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("expected path traversal error, got: %v", err)
			}
		})
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		path     string
		expected bool
	}{
		{"same dir", "/base", "/base", true},
		{"subdir", "/base", "/base/sub", true},
		{"parent escape", "/base", "/base/../etc", false},
		{"absolute escape", "/base", "/etc/passwd", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSubPath(tc.base, tc.path); got != tc.expected {
				t.Errorf("isSubPath(%q, %q) = %v, want %v", tc.base, tc.path, got, tc.expected)
			}
		})
	}
}

func TestResizeNearest(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.Set(0, 0, color.RGBA{R: 255, A: 255})
	src.Set(3, 1, color.RGBA{B: 255, A: 255})
	dst := resizeNearest(src, 2, 1)
	if dst.Bounds().Dx() != 2 || dst.Bounds().Dy() != 1 {
		t.Fatalf("unexpected size %v", dst.Bounds())
	}
}
