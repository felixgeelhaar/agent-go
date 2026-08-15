package pdf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack(t.TempDir())
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Metadata["status"] == "stub" {
		t.Fatal("expected no stub metadata")
	}
	if p.Name != "pdf" {
		t.Errorf("expected pack name %q, got %q", "pdf", p.Name)
	}
	if strings.Contains(p.Description, "STUB") {
		t.Errorf("description should not mention STUB: %q", p.Description)
	}
	if got := len(p.Tools); got != 4 {
		t.Errorf("expected 4 tools, got %d", got)
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
		"pdf_extract_text": func(a tool.Annotations) bool { return a.ReadOnly && a.Cacheable },
		"pdf_metadata":     func(a tool.Annotations) bool { return a.ReadOnly && a.Cacheable },
		"pdf_merge":        func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
		"pdf_split":        func(a tool.Annotations) bool { return a.Idempotent && a.RiskLevel == tool.RiskLow },
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

func getTool(t *testing.T, baseDir, name string) tool.Tool {
	t.Helper()
	tt, ok := Pack(baseDir).GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found", name)
	}
	return tt
}

func execTool(t *testing.T, tt tool.Tool, input any) json.RawMessage {
	t.Helper()
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	result, err := tt.Execute(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("tool %q failed: %v", tt.Name(), err)
	}
	return result.Output
}

func writeTestPDF(t *testing.T, path, title string, pages ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	doc := gofpdf.New("P", "mm", "A4", "")
	if title != "" {
		doc.SetTitle(title, false)
	}
	doc.SetAuthor("pack-pdf-test", false)
	for _, pageText := range pages {
		doc.AddPage()
		doc.SetFont("Helvetica", "", 14)
		doc.Cell(40, 10, pageText)
	}
	if err := doc.OutputFileAndClose(path); err != nil {
		t.Fatalf("write test PDF: %v", err)
	}
}

func TestExtractTextAndMetadata(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPDF(t, filepath.Join(baseDir, "doc.pdf"), "My Title", "Hello PDF World", "Second Page")

	out := execTool(t, getTool(t, baseDir, "pdf_extract_text"), map[string]any{"path": "doc.pdf"})
	var textOut extractTextOutput
	if err := json.Unmarshal(out, &textOut); err != nil {
		t.Fatalf("unmarshal extract: %v", err)
	}
	if textOut.PageCount != 2 {
		t.Errorf("expected 2 pages, got %d", textOut.PageCount)
	}
	if !strings.Contains(textOut.Text, "Hello PDF World") {
		t.Errorf("expected text to contain Hello PDF World, got %q", textOut.Text)
	}
	if !strings.Contains(textOut.Text, "Second Page") {
		t.Errorf("expected text to contain Second Page, got %q", textOut.Text)
	}

	out = execTool(t, getTool(t, baseDir, "pdf_metadata"), map[string]any{"path": "doc.pdf"})
	var meta metadataOutput
	if err := json.Unmarshal(out, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.PageCount != 2 {
		t.Errorf("expected page_count 2, got %d", meta.PageCount)
	}
	if meta.Size <= 0 {
		t.Errorf("expected positive size, got %d", meta.Size)
	}
	if meta.Title != "My Title" {
		t.Errorf("expected title %q, got %q", "My Title", meta.Title)
	}
	if meta.Author != "pack-pdf-test" {
		t.Errorf("expected author pack-pdf-test, got %q", meta.Author)
	}
}

func TestMergeAndSplit(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPDF(t, filepath.Join(baseDir, "a.pdf"), "A", "Alpha")
	writeTestPDF(t, filepath.Join(baseDir, "b.pdf"), "B", "Beta", "Gamma")

	out := execTool(t, getTool(t, baseDir, "pdf_merge"), map[string]any{
		"paths":  []string{"a.pdf", "b.pdf"},
		"output": "merged.pdf",
	})
	var mergeOut mergeOutput
	if err := json.Unmarshal(out, &mergeOut); err != nil {
		t.Fatalf("unmarshal merge: %v", err)
	}
	if mergeOut.PageCount != 3 {
		t.Errorf("expected merged page_count 3, got %d", mergeOut.PageCount)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "merged.pdf")); err != nil {
		t.Fatalf("merged.pdf missing: %v", err)
	}

	out = execTool(t, getTool(t, baseDir, "pdf_split"), map[string]any{
		"path":       "merged.pdf",
		"output":     "part.pdf",
		"start_page": 2,
		"end_page":   3,
	})
	var splitOut splitOutput
	if err := json.Unmarshal(out, &splitOut); err != nil {
		t.Fatalf("unmarshal split: %v", err)
	}
	if splitOut.PageCount != 2 {
		t.Errorf("expected split page_count 2, got %d", splitOut.PageCount)
	}
	if splitOut.Pages != "2-3" {
		t.Errorf("expected pages 2-3, got %q", splitOut.Pages)
	}

	out = execTool(t, getTool(t, baseDir, "pdf_extract_text"), map[string]any{"path": "part.pdf"})
	var textOut extractTextOutput
	if err := json.Unmarshal(out, &textOut); err != nil {
		t.Fatalf("unmarshal split extract: %v", err)
	}
	if !strings.Contains(textOut.Text, "Beta") || !strings.Contains(textOut.Text, "Gamma") {
		t.Errorf("expected Beta and Gamma in split PDF, got %q", textOut.Text)
	}
	if strings.Contains(textOut.Text, "Alpha") {
		t.Errorf("did not expect Alpha in split pages 2-3, got %q", textOut.Text)
	}

	out = execTool(t, getTool(t, baseDir, "pdf_split"), map[string]any{
		"path":   "merged.pdf",
		"output": "page1.pdf",
		"pages":  "1",
	})
	if err := json.Unmarshal(out, &splitOut); err != nil {
		t.Fatalf("unmarshal pages split: %v", err)
	}
	if splitOut.PageCount != 1 {
		t.Errorf("expected 1 page, got %d", splitOut.PageCount)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	baseDir := t.TempDir()
	writeTestPDF(t, filepath.Join(baseDir, "ok.pdf"), "OK", "safe")

	cases := []struct {
		name  string
		tool  string
		input map[string]any
	}{
		{"extract", "pdf_extract_text", map[string]any{"path": "../../etc/passwd"}},
		{"metadata", "pdf_metadata", map[string]any{"path": "/etc/passwd"}},
		{"merge-src", "pdf_merge", map[string]any{"paths": []string{"../escape.pdf"}, "output": "out.pdf"}},
		{"merge-out", "pdf_merge", map[string]any{"paths": []string{"ok.pdf"}, "output": "../out.pdf"}},
		{"split-src", "pdf_split", map[string]any{"path": "../ok.pdf", "output": "out.pdf", "pages": "1"}},
		{"split-out", "pdf_split", map[string]any{"path": "ok.pdf", "output": "../out.pdf", "pages": "1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tt := getTool(t, baseDir, tc.tool)
			input, _ := json.Marshal(tc.input)
			_, err := tt.Execute(context.Background(), input)
			if err == nil {
				t.Fatal("expected path traversal error")
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
