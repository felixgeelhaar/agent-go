package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

func TestNew_RequiresProvider(t *testing.T) {
	t.Parallel()

	_, err := New(nil)
	if err == nil {
		t.Error("New(nil) expected error, got nil")
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, err := New(provider)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if pack.Name != "copilot" {
		t.Errorf("pack.Name = %s, want copilot", pack.Name)
	}

	if pack.Version != "1.0.0" {
		t.Errorf("pack.Version = %s, want 1.0.0", pack.Version)
	}

	// Should have 4 core tools by default
	if len(pack.Tools) != 4 {
		t.Errorf("len(pack.Tools) = %d, want 4", len(pack.Tools))
	}

	// Verify core tools exist
	expectedTools := []string{"copilot_complete", "copilot_explain", "copilot_suggest_fix", "copilot_generate_tests"}
	for _, name := range expectedTools {
		if _, ok := pack.GetTool(name); !ok {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestNew_WithReview(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, err := New(provider, WithReview())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if len(pack.Tools) != 5 {
		t.Errorf("len(pack.Tools) = %d, want 5", len(pack.Tools))
	}

	if _, ok := pack.GetTool("copilot_review"); !ok {
		t.Error("copilot_review tool not found")
	}

	// Should be allowed in decide state
	allowed := pack.AllowedInState(agent.StateDecide)
	found := false
	for _, name := range allowed {
		if name == "copilot_review" {
			found = true
			break
		}
	}
	if !found {
		t.Error("copilot_review should be allowed in decide state")
	}
}

func TestNew_WithRefactor(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, err := New(provider, WithRefactor())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if len(pack.Tools) != 5 {
		t.Errorf("len(pack.Tools) = %d, want 5", len(pack.Tools))
	}

	if _, ok := pack.GetTool("copilot_refactor"); !ok {
		t.Error("copilot_refactor tool not found")
	}
}

func TestNew_AllOptions(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, err := New(provider,
		WithTimeout(30*time.Second),
		WithReview(),
		WithRefactor(),
		WithDefaultLanguage("python"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if len(pack.Tools) != 6 {
		t.Errorf("len(pack.Tools) = %d, want 6", len(pack.Tools))
	}
}

func TestNew_Eligibility(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, err := New(provider, WithReview())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// All tools should be allowed in explore
	exploreTools := pack.AllowedInState(agent.StateExplore)
	if len(exploreTools) < 4 {
		t.Errorf("expected at least 4 tools in explore state, got %d", len(exploreTools))
	}

	// Explain should be allowed in validate
	validateTools := pack.AllowedInState(agent.StateValidate)
	found := false
	for _, name := range validateTools {
		if name == "copilot_explain" {
			found = true
			break
		}
	}
	if !found {
		t.Error("copilot_explain should be allowed in validate state")
	}
}

func TestMemoryProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	if provider.Name() != "memory" {
		t.Errorf("Name() = %s, want memory", provider.Name())
	}
}

func TestMemoryProvider_Complete(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	ctx := context.Background()

	resp, err := provider.Complete(ctx, CompleteRequest{
		Code:     "func hello() {",
		Language: "go",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Completion == "" {
		t.Error("Complete() returned empty completion")
	}

	if resp.Confidence <= 0 {
		t.Error("Complete() returned zero confidence")
	}
}

func TestMemoryProvider_Explain(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	ctx := context.Background()

	resp, err := provider.Explain(ctx, ExplainRequest{
		Code:     "func hello() { return \"world\" }",
		Language: "go",
	})
	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}

	if resp.Explanation == "" {
		t.Error("Explain() returned empty explanation")
	}

	if resp.Summary == "" {
		t.Error("Explain() returned empty summary")
	}
}

func TestMemoryProvider_Review(t *testing.T) {
	t.Parallel()

	t.Run("clean code", func(t *testing.T) {
		t.Parallel()

		provider := NewMemoryProvider()
		ctx := context.Background()

		resp, err := provider.Review(ctx, ReviewRequest{
			Code:     "func clean() { return nil }",
			Language: "go",
		})
		if err != nil {
			t.Fatalf("Review() error = %v", err)
		}

		if resp.OverallScore <= 0 {
			t.Error("Review() returned zero score")
		}
	})

	t.Run("code with TODO", func(t *testing.T) {
		t.Parallel()

		provider := NewMemoryProvider()
		ctx := context.Background()

		resp, err := provider.Review(ctx, ReviewRequest{
			Code:     "func todo() { // TODO: fix this }",
			Language: "go",
		})
		if err != nil {
			t.Fatalf("Review() error = %v", err)
		}

		if len(resp.Issues) == 0 {
			t.Error("Review() should find TODO issue")
		}
	})
}

func TestMemoryProvider_SuggestFix(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	ctx := context.Background()

	resp, err := provider.SuggestFix(ctx, SuggestFixRequest{
		Code:     "func broken() { nil.method() }",
		Error:    "nil pointer dereference",
		Language: "go",
	})
	if err != nil {
		t.Fatalf("SuggestFix() error = %v", err)
	}

	if resp.Fix == "" {
		t.Error("SuggestFix() returned empty fix")
	}

	if resp.Explanation == "" {
		t.Error("SuggestFix() returned empty explanation")
	}
}

func TestMemoryProvider_GenerateTests(t *testing.T) {
	t.Parallel()

	t.Run("go code", func(t *testing.T) {
		t.Parallel()

		provider := NewMemoryProvider()
		ctx := context.Background()

		resp, err := provider.GenerateTests(ctx, GenerateTestsRequest{
			Code:     "func Add(a, b int) int { return a + b }",
			Language: "go",
		})
		if err != nil {
			t.Fatalf("GenerateTests() error = %v", err)
		}

		if resp.Tests == "" {
			t.Error("GenerateTests() returned empty tests")
		}

		if resp.CoverageEstimate <= 0 {
			t.Error("GenerateTests() returned zero coverage estimate")
		}
	})

	t.Run("python code", func(t *testing.T) {
		t.Parallel()

		provider := NewMemoryProvider()
		ctx := context.Background()

		resp, err := provider.GenerateTests(ctx, GenerateTestsRequest{
			Code:     "def add(a, b): return a + b",
			Language: "python",
		})
		if err != nil {
			t.Fatalf("GenerateTests() error = %v", err)
		}

		if resp.Tests == "" {
			t.Error("GenerateTests() returned empty tests")
		}
	})
}

func TestMemoryProvider_Refactor(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	ctx := context.Background()

	resp, err := provider.Refactor(ctx, RefactorRequest{
		Code:     "func mess() { if x { if y { if z { } } } }",
		Language: "go",
		Goal:     "simplify",
	})
	if err != nil {
		t.Fatalf("Refactor() error = %v", err)
	}

	if resp.RefactoredCode == "" {
		t.Error("Refactor() returned empty code")
	}

	if len(resp.Changes) == 0 {
		t.Error("Refactor() returned no changes")
	}
}

func TestMemoryProvider_CustomHandlers(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	expectedError := errors.New("custom error")

	provider.CompleteFunc = func(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
		return CompleteResponse{}, expectedError
	}

	ctx := context.Background()
	_, err := provider.Complete(ctx, CompleteRequest{Code: "test"})
	if !errors.Is(err, expectedError) {
		t.Errorf("Complete() error = %v, want %v", err, expectedError)
	}
}

func TestCompleteTool(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, _ := New(provider)

	tool, ok := pack.GetTool("copilot_complete")
	if !ok {
		t.Fatal("copilot_complete tool not found")
	}

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(CompleteRequest{
			Code:     "func hello() {",
			Language: "go",
		})

		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if len(result.Output) == 0 {
			t.Error("Execute() returned empty output")
		}
	})

	t.Run("missing code", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(CompleteRequest{
			Language: "go",
		})

		_, err := tool.Execute(context.Background(), input)
		if err == nil {
			t.Error("Execute() expected error for missing code")
		}
	})
}

func TestExplainTool(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, _ := New(provider)

	tool, ok := pack.GetTool("copilot_explain")
	if !ok {
		t.Fatal("copilot_explain tool not found")
	}

	input, _ := json.Marshal(ExplainRequest{
		Code: "func main() { fmt.Println(\"Hello\") }",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp ExplainResponse
	if err := json.Unmarshal(result.Output, &resp); err != nil {
		t.Fatalf("Unmarshal response error = %v", err)
	}

	if resp.Explanation == "" {
		t.Error("Explanation is empty")
	}
}

func TestSuggestFixTool(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, _ := New(provider)

	tool, ok := pack.GetTool("copilot_suggest_fix")
	if !ok {
		t.Fatal("copilot_suggest_fix tool not found")
	}

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(SuggestFixRequest{
			Code:  "func broken() { return nil.method() }",
			Error: "nil pointer",
		})

		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if len(result.Output) == 0 {
			t.Error("Execute() returned empty output")
		}
	})

	t.Run("missing error", func(t *testing.T) {
		t.Parallel()

		input, _ := json.Marshal(SuggestFixRequest{
			Code: "func broken() { }",
		})

		_, err := tool.Execute(context.Background(), input)
		if err == nil {
			t.Error("Execute() expected error for missing error description")
		}
	})
}

func TestGenerateTestsTool(t *testing.T) {
	t.Parallel()

	provider := NewMemoryProvider()
	pack, _ := New(provider)

	tool, ok := pack.GetTool("copilot_generate_tests")
	if !ok {
		t.Fatal("copilot_generate_tests tool not found")
	}

	input, _ := json.Marshal(GenerateTestsRequest{
		Code:     "func Add(a, b int) int { return a + b }",
		Language: "go",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resp GenerateTestsResponse
	if err := json.Unmarshal(result.Output, &resp); err != nil {
		t.Fatalf("Unmarshal response error = %v", err)
	}

	if resp.Tests == "" {
		t.Error("Tests is empty")
	}
}
