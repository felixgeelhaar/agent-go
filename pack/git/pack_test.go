package git

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-pack-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize repository
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create initial file
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository\n"), 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add and commit
	worktree, err := repo.Worktree()
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to get worktree: %v", err)
	}

	_, err = worktree.Add("README.md")
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to commit: %v", err)
	}

	return dir
}

func TestNew(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}

	if p.Name != "git" {
		t.Errorf("expected name 'git', got '%s'", p.Name)
	}
}

func TestNewInvalidRepo(t *testing.T) {
	_, err := New("/nonexistent/path")
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}

func TestNewWithOptions(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir,
		WithWriteAccess(),
		WithCheckoutAccess(),
		WithAuthor("Test Author", "author@test.com"),
		WithMaxLogEntries(50),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestStatusTool(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_status")
	if !ok {
		t.Fatal("git_status tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Branch == "" {
		t.Error("expected branch name")
	}

	if !out.IsClean {
		t.Error("expected clean repository")
	}
}

func TestStatusToolWithUnstagedChanges(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Modify a file
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_status")
	if !ok {
		t.Fatal("git_status tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.IsClean {
		t.Error("expected dirty repository")
	}

	if len(out.Unstaged) == 0 {
		t.Error("expected unstaged changes")
	}
}

func TestLogTool(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_log")
	if !ok {
		t.Fatal("git_log tool not found")
	}

	input, _ := json.Marshal(logInput{Limit: 10})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}

	var out logOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 commit")
	}

	if len(out.Commits) < 1 {
		t.Error("expected at least 1 commit in list")
	}

	// Check first commit has expected fields
	commit := out.Commits[0]
	if commit.Hash == "" {
		t.Error("expected commit hash")
	}
	if commit.Message == "" {
		t.Error("expected commit message")
	}
}

func TestDiffTool(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Modify a file
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified Content\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_diff")
	if !ok {
		t.Fatal("git_diff tool not found")
	}

	input, _ := json.Marshal(diffInput{Staged: false})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	var out diffOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(out.Changes) == 0 {
		t.Error("expected changes")
	}

	if out.Summary == "" {
		t.Error("expected summary")
	}
}

func TestBranchTool(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_branch")
	if !ok {
		t.Fatal("git_branch tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("branch failed: %v", err)
	}

	var out branchOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should have at least master/main branch
	if out.Count < 1 {
		t.Error("expected at least 1 branch")
	}

	if out.Current == "" {
		t.Error("expected current branch")
	}
}

func TestAddToolWithWriteAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create a new file
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("New content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_add")
	if !ok {
		t.Fatal("git_add tool not found")
	}

	input, _ := json.Marshal(addInput{Paths: []string{"new.txt"}})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	var out addOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 1 {
		t.Errorf("expected 1 file added, got %d", out.Count)
	}
}

func TestAddToolAll(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create multiple new files
	for i := 0; i < 3; i++ {
		newFile := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(newFile, []byte("content\n"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_add")
	if !ok {
		t.Fatal("git_add tool not found")
	}

	input, _ := json.Marshal(addInput{All: true})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("add all failed: %v", err)
	}

	var out addOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 file added")
	}
}

func TestCommitToolWithWriteAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create and stage a new file
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("New content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()
	worktree.Add("new.txt")

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_commit")
	if !ok {
		t.Fatal("git_commit tool not found")
	}

	input, _ := json.Marshal(commitInput{
		Message: "Add new file",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	var out commitOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Hash == "" {
		t.Error("expected commit hash")
	}

	if out.Message != "Add new file" {
		t.Errorf("expected message 'Add new file', got '%s'", out.Message)
	}
}

func TestCommitToolEmptyMessage(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_commit")
	if !ok {
		t.Fatal("git_commit tool not found")
	}

	input, _ := json.Marshal(commitInput{Message: ""})
	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty commit message")
	}
}

func TestCheckoutToolWithCheckoutAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithCheckoutAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_checkout")
	if !ok {
		t.Fatal("git_checkout tool not found")
	}

	// Create and switch to new branch
	input, _ := json.Marshal(checkoutInput{
		Branch: "feature-test",
		Create: true,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	var out checkoutOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Branch != "feature-test" {
		t.Errorf("expected branch 'feature-test', got '%s'", out.Branch)
	}

	if !out.Created {
		t.Error("expected created to be true")
	}
}

func TestCheckoutToolEmptyBranch(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithCheckoutAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_checkout")
	if !ok {
		t.Fatal("git_checkout tool not found")
	}

	input, _ := json.Marshal(checkoutInput{Branch: ""})
	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty branch name")
	}
}

func TestAddToolWithoutWriteAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir) // No write access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("git_add")
	if ok {
		t.Error("expected git_add tool to not exist without write access")
	}
}

func TestCheckoutToolWithoutCheckoutAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir) // No checkout access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("git_checkout")
	if ok {
		t.Error("expected git_checkout tool to not exist without checkout access")
	}
}

func TestToolAnnotations(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithWriteAccess(), WithCheckoutAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	// Check status tool is read-only
	if statusTool, ok := p.GetTool("git_status"); ok {
		annotations := statusTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("git_status should be read-only")
		}
	}

	// Check log tool is read-only and cacheable
	if logTool, ok := p.GetTool("git_log"); ok {
		annotations := logTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("git_log should be read-only")
		}
		if !annotations.Cacheable {
			t.Error("git_log should be cacheable")
		}
	}

	// Check add tool is destructive
	if addTool, ok := p.GetTool("git_add"); ok {
		annotations := addTool.Annotations()
		if !annotations.Destructive {
			t.Error("git_add should be destructive")
		}
	}

	// Check commit tool is destructive
	if commitTool, ok := p.GetTool("git_commit"); ok {
		annotations := commitTool.Annotations()
		if !annotations.Destructive {
			t.Error("git_commit should be destructive")
		}
	}
}

func TestNewWithPushAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithPushAccess())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestStatusCodeToString(t *testing.T) {
	tests := []struct {
		code     git.StatusCode
		expected string
	}{
		{git.Unmodified, "unmodified"},
		{git.Untracked, "untracked"},
		{git.Modified, "modified"},
		{git.Added, "added"},
		{git.Deleted, "deleted"},
		{git.Renamed, "renamed"},
		{git.Copied, "copied"},
		{git.UpdatedButUnmerged, "unmerged"},
		{git.StatusCode('X'), "unknown"}, // Unknown code
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := statusCodeToString(tt.code)
			if result != tt.expected {
				t.Errorf("statusCodeToString(%v) = %s, want %s", tt.code, result, tt.expected)
			}
		})
	}
}

func TestLogToolWithBranch(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create a new branch with some commits
	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()

	// Create and checkout feature branch
	err := worktree.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/feature",
		Create: true,
	})
	if err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Create a commit on feature branch
	testFile := filepath.Join(dir, "feature.txt")
	if err := os.WriteFile(testFile, []byte("Feature content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	worktree.Add("feature.txt")
	worktree.Commit("Feature commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_log")
	if !ok {
		t.Fatal("git_log tool not found")
	}

	// Test with branch filter
	input, _ := json.Marshal(logInput{Branch: "feature", Limit: 10})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("log with branch failed: %v", err)
	}

	var out logOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 commit on feature branch")
	}
}

func TestLogToolWithInvalidBranch(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_log")
	if !ok {
		t.Fatal("git_log tool not found")
	}

	input, _ := json.Marshal(logInput{Branch: "nonexistent-branch"})
	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid branch")
	}
}

func TestLogToolWithPath(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create additional commits with different files
	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()

	// Create a file in a subdirectory
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("Content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	worktree.Add("subdir/file.txt")
	worktree.Commit("Add subdir file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_log")
	if !ok {
		t.Fatal("git_log tool not found")
	}

	// Test with path filter
	input, _ := json.Marshal(logInput{Path: "subdir", Limit: 10})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("log with path failed: %v", err)
	}

	var out logOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should have commits affecting subdir
	if out.Count < 1 {
		t.Error("expected at least 1 commit for path filter")
	}
}

func TestDiffToolStaged(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create and stage a new file
	newFile := filepath.Join(dir, "staged.txt")
	if err := os.WriteFile(newFile, []byte("Staged content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()
	worktree.Add("staged.txt")

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_diff")
	if !ok {
		t.Fatal("git_diff tool not found")
	}

	// Test staged changes
	input, _ := json.Marshal(diffInput{Staged: true})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("diff staged failed: %v", err)
	}

	var out diffOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(out.Changes) == 0 {
		t.Error("expected staged changes")
	}

	if !contains(out.Summary, "staged") {
		t.Error("expected summary to contain 'staged'")
	}
}

func TestDiffToolWithPath(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Modify README.md
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Create another modified file
	otherFile := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("Other\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_diff")
	if !ok {
		t.Fatal("git_diff tool not found")
	}

	// Test with path filter
	input, _ := json.Marshal(diffInput{Path: "README"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("diff with path failed: %v", err)
	}

	var out diffOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should only have README change, not other.txt
	for _, change := range out.Changes {
		if change.Path == "other.txt" {
			t.Error("expected path filter to exclude other.txt")
		}
	}
}

func TestCommitToolWithCustomAuthor(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create and stage a new file
	newFile := filepath.Join(dir, "authored.txt")
	if err := os.WriteFile(newFile, []byte("Custom author content\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()
	worktree.Add("authored.txt")

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_commit")
	if !ok {
		t.Fatal("git_commit tool not found")
	}

	input, _ := json.Marshal(commitInput{
		Message:     "Custom author commit",
		AuthorName:  "Custom Author",
		AuthorEmail: "custom@example.com",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	var out commitOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Hash == "" {
		t.Error("expected commit hash")
	}

	// Verify the author was set correctly
	commit, err := repo.CommitObject(plumbing.NewHash(out.Hash))
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	if commit.Author.Name != "Custom Author" {
		t.Errorf("expected author name 'Custom Author', got '%s'", commit.Author.Name)
	}
	if commit.Author.Email != "custom@example.com" {
		t.Errorf("expected author email 'custom@example.com', got '%s'", commit.Author.Email)
	}
}

func TestStatusToolWithStagedChanges(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create and stage a new file (Added status)
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("New file\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	repo, _ := git.PlainOpen(dir)
	worktree, _ := repo.Worktree()
	worktree.Add("new.txt")

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_status")
	if !ok {
		t.Fatal("git_status tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(out.Staged) == 0 {
		t.Error("expected staged changes")
	}

	// Verify the status is "added"
	found := false
	for _, s := range out.Staged {
		if s.Path == "new.txt" && s.Status == "added" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected new.txt to be staged with 'added' status")
	}
}

func TestStatusToolWithUntrackedFiles(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Create an untracked file
	untrackedFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("Untracked\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_status")
	if !ok {
		t.Fatal("git_status tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(out.Untracked) == 0 {
		t.Error("expected untracked files")
	}

	found := false
	for _, f := range out.Untracked {
		if f == "untracked.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected untracked.txt in untracked files")
	}
}

func TestStatusToolWithDeletedFile(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Delete the README.md file
	testFile := filepath.Join(dir, "README.md")
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_status")
	if !ok {
		t.Fatal("git_status tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Check for deleted status in unstaged changes
	found := false
	for _, s := range out.Unstaged {
		if s.Path == "README.md" && s.Status == "deleted" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected README.md to have 'deleted' status")
	}
}

func TestLogToolWithInvalidJSON(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_log")
	if !ok {
		t.Fatal("git_log tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestDiffToolWithInvalidJSON(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_diff")
	if !ok {
		t.Fatal("git_diff tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestAddToolWithInvalidJSON(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_add")
	if !ok {
		t.Fatal("git_add tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestCommitToolWithInvalidJSON(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_commit")
	if !ok {
		t.Fatal("git_commit tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestCheckoutToolWithInvalidJSON(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithCheckoutAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("git_checkout")
	if !ok {
		t.Fatal("git_checkout tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestNewWithWriteAndCheckoutAccess(t *testing.T) {
	dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	p, err := New(dir, WithWriteAccess(), WithCheckoutAccess())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all tools
	tools := []string{"git_status", "git_log", "git_diff", "git_branch", "git_add", "git_commit", "git_checkout"}
	for _, name := range tools {
		if _, ok := p.GetTool(name); !ok {
			t.Errorf("expected tool %s to exist", name)
		}
	}
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
