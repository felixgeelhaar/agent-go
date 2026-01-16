// Package git provides git repository operation tools.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Config configures the git pack.
type Config struct {
	// RepoPath is the path to the git repository.
	RepoPath string

	// AllowWrite enables write operations (add, commit).
	AllowWrite bool

	// AllowCheckout enables branch switching.
	AllowCheckout bool

	// AllowPush enables push operations.
	AllowPush bool

	// DefaultAuthorName is the default commit author name.
	DefaultAuthorName string

	// DefaultAuthorEmail is the default commit author email.
	DefaultAuthorEmail string

	// MaxLogEntries limits the number of log entries returned.
	MaxLogEntries int
}

// Option configures the git pack.
type Option func(*Config)

// WithWriteAccess enables write operations (add, commit).
func WithWriteAccess() Option {
	return func(c *Config) {
		c.AllowWrite = true
	}
}

// WithCheckoutAccess enables branch switching.
func WithCheckoutAccess() Option {
	return func(c *Config) {
		c.AllowCheckout = true
	}
}

// WithPushAccess enables push operations.
func WithPushAccess() Option {
	return func(c *Config) {
		c.AllowPush = true
	}
}

// WithAuthor sets the default commit author.
func WithAuthor(name, email string) Option {
	return func(c *Config) {
		c.DefaultAuthorName = name
		c.DefaultAuthorEmail = email
	}
}

// WithMaxLogEntries sets the maximum log entries returned.
func WithMaxLogEntries(max int) Option {
	return func(c *Config) {
		c.MaxLogEntries = max
	}
}

// New creates the git pack.
func New(repoPath string, opts ...Option) (*pack.Pack, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repo path: %w", err)
	}

	// Open repository to verify it exists
	repo, err := git.PlainOpen(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	cfg := Config{
		RepoPath:           absPath,
		AllowWrite:         false,
		AllowCheckout:      false,
		AllowPush:          false,
		DefaultAuthorName:  "Agent",
		DefaultAuthorEmail: "agent@local",
		MaxLogEntries:      100,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Create pack context with repo reference
	packCtx := &packContext{
		repo: repo,
		cfg:  &cfg,
	}

	builder := pack.NewBuilder("git").
		WithDescription("Git repository operations").
		WithVersion("1.0.0").
		AddTools(
			statusTool(packCtx),
			logTool(packCtx),
			diffTool(packCtx),
			branchTool(packCtx),
		).
		AllowInState(agent.StateExplore, "git_status", "git_log", "git_diff", "git_branch").
		AllowInState(agent.StateValidate, "git_status", "git_log", "git_diff", "git_branch")

	// Add write tools if enabled
	if cfg.AllowWrite {
		builder = builder.AddTools(addTool(packCtx), commitTool(packCtx))
		builder = builder.AllowInState(agent.StateAct, "git_status", "git_log", "git_diff", "git_branch", "git_add", "git_commit")
	} else {
		builder = builder.AllowInState(agent.StateAct, "git_status", "git_log", "git_diff", "git_branch")
	}

	// Add checkout if enabled
	if cfg.AllowCheckout {
		builder = builder.AddTools(checkoutTool(packCtx))
		// Update StateAct to include checkout
		existingAllowed := builder.Build().Eligibility[agent.StateAct]
		builder = pack.NewBuilder("git").
			WithDescription("Git repository operations").
			WithVersion("1.0.0")

		// Rebuild with all tools
		tools := []tool.Tool{
			statusTool(packCtx),
			logTool(packCtx),
			diffTool(packCtx),
			branchTool(packCtx),
		}
		if cfg.AllowWrite {
			tools = append(tools, addTool(packCtx), commitTool(packCtx))
		}
		if cfg.AllowCheckout {
			tools = append(tools, checkoutTool(packCtx))
		}

		builder = builder.AddTools(tools...).
			AllowInState(agent.StateExplore, "git_status", "git_log", "git_diff", "git_branch").
			AllowInState(agent.StateValidate, "git_status", "git_log", "git_diff", "git_branch")

		// Build act state tools (copy to avoid modifying original slice)
		actTools := make([]string, len(existingAllowed)+1)
		copy(actTools, existingAllowed)
		actTools[len(existingAllowed)] = "git_checkout"
		builder = builder.AllowInState(agent.StateAct, actTools...)
	}

	return builder.Build(), nil
}

// packContext holds the repository reference for tools.
type packContext struct {
	repo *git.Repository
	cfg  *Config
}

// statusOutput is the output for the git_status tool.
type statusOutput struct {
	Branch      string         `json:"branch"`
	Staged      []fileStatus   `json:"staged"`
	Unstaged    []fileStatus   `json:"unstaged"`
	Untracked   []string       `json:"untracked"`
	IsClean     bool           `json:"is_clean"`
	Ahead       int            `json:"ahead,omitempty"`
	Behind      int            `json:"behind,omitempty"`
}

type fileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func statusTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_status").
		WithDescription("Get repository status including staged, unstaged, and untracked files").
		ReadOnly().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			worktree, err := ctx.repo.Worktree()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get worktree: %w", err)
			}

			status, err := worktree.Status()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get status: %w", err)
			}

			// Get current branch
			head, err := ctx.repo.Head()
			branch := "HEAD (detached)"
			if err == nil && head.Name().IsBranch() {
				branch = head.Name().Short()
			}

			out := statusOutput{
				Branch:    branch,
				Staged:    make([]fileStatus, 0),
				Unstaged:  make([]fileStatus, 0),
				Untracked: make([]string, 0),
				IsClean:   status.IsClean(),
			}

			for path, s := range status {
				// Staged changes (index)
				if s.Staging != git.Unmodified && s.Staging != git.Untracked {
					out.Staged = append(out.Staged, fileStatus{
						Path:   path,
						Status: statusCodeToString(s.Staging),
					})
				}

				// Unstaged changes (worktree)
				if s.Worktree != git.Unmodified && s.Worktree != git.Untracked {
					out.Unstaged = append(out.Unstaged, fileStatus{
						Path:   path,
						Status: statusCodeToString(s.Worktree),
					})
				}

				// Untracked files
				if s.Worktree == git.Untracked {
					out.Untracked = append(out.Untracked, path)
				}
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func statusCodeToString(code git.StatusCode) string {
	switch code {
	case git.Unmodified:
		return "unmodified"
	case git.Untracked:
		return "untracked"
	case git.Modified:
		return "modified"
	case git.Added:
		return "added"
	case git.Deleted:
		return "deleted"
	case git.Renamed:
		return "renamed"
	case git.Copied:
		return "copied"
	case git.UpdatedButUnmerged:
		return "unmerged"
	default:
		return "unknown"
	}
}

// logInput is the input for the git_log tool.
type logInput struct {
	Limit  int    `json:"limit,omitempty"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path,omitempty"`
}

// logOutput is the output for the git_log tool.
type logOutput struct {
	Commits []commitInfo `json:"commits"`
	Count   int          `json:"count"`
}

type commitInfo struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Message     string    `json:"message"`
	Author      string    `json:"author"`
	AuthorEmail string    `json:"author_email"`
	Date        time.Time `json:"date"`
}

func logTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_log").
		WithDescription("Get commit history").
		ReadOnly().
		Cacheable().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			var in logInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			limit := ctx.cfg.MaxLogEntries
			if in.Limit > 0 && in.Limit < limit {
				limit = in.Limit
			}

			opts := &git.LogOptions{}

			// If branch specified, get its reference
			if in.Branch != "" {
				ref, err := ctx.repo.Reference(plumbing.NewBranchReferenceName(in.Branch), true)
				if err != nil {
					return tool.Result{}, fmt.Errorf("branch not found: %w", err)
				}
				opts.From = ref.Hash()
			}

			// If path specified, filter by path
			if in.Path != "" {
				opts.PathFilter = func(p string) bool {
					return strings.HasPrefix(p, in.Path)
				}
			}

			logIter, err := ctx.repo.Log(opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get log: %w", err)
			}
			defer logIter.Close()

			out := logOutput{
				Commits: make([]commitInfo, 0),
			}

			count := 0
			err = logIter.ForEach(func(commit *object.Commit) error {
				if count >= limit {
					return errors.New("limit reached")
				}
				count++

				out.Commits = append(out.Commits, commitInfo{
					Hash:        commit.Hash.String(),
					ShortHash:   commit.Hash.String()[:7],
					Message:     strings.TrimSpace(commit.Message),
					Author:      commit.Author.Name,
					AuthorEmail: commit.Author.Email,
					Date:        commit.Author.When,
				})
				return nil
			})
			// Ignore "limit reached" error
			if err != nil && err.Error() != "limit reached" {
				return tool.Result{}, fmt.Errorf("failed to iterate log: %w", err)
			}

			out.Count = len(out.Commits)

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// diffInput is the input for the git_diff tool.
type diffInput struct {
	Staged bool   `json:"staged,omitempty"`
	Path   string `json:"path,omitempty"`
}

// diffOutput is the output for the git_diff tool.
type diffOutput struct {
	Changes []diffChange `json:"changes"`
	Summary string       `json:"summary"`
}

type diffChange struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

func diffTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_diff").
		WithDescription("Show changes in working directory or staged changes").
		ReadOnly().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			var in diffInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			worktree, err := ctx.repo.Worktree()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get worktree: %w", err)
			}

			status, err := worktree.Status()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get status: %w", err)
			}

			out := diffOutput{
				Changes: make([]diffChange, 0),
			}

			for path, s := range status {
				// Filter by path if specified
				if in.Path != "" && !strings.HasPrefix(path, in.Path) {
					continue
				}

				var change *diffChange
				if in.Staged {
					// Show staged changes
					if s.Staging != git.Unmodified && s.Staging != git.Untracked {
						change = &diffChange{
							Path: path,
							Type: statusCodeToString(s.Staging),
						}
					}
				} else {
					// Show unstaged changes
					if s.Worktree != git.Unmodified {
						change = &diffChange{
							Path: path,
							Type: statusCodeToString(s.Worktree),
						}
					}
				}

				if change != nil {
					out.Changes = append(out.Changes, *change)
				}
			}

			// Generate summary
			var buf bytes.Buffer
			buf.WriteString(fmt.Sprintf("%d files changed", len(out.Changes)))
			if in.Staged {
				buf.WriteString(" (staged)")
			} else {
				buf.WriteString(" (unstaged)")
			}
			out.Summary = buf.String()

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// branchOutput is the output for the git_branch tool.
type branchOutput struct {
	Current  string       `json:"current"`
	Branches []branchInfo `json:"branches"`
	Count    int          `json:"count"`
}

type branchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	IsRemote  bool   `json:"is_remote"`
}

func branchTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_branch").
		WithDescription("List branches").
		ReadOnly().
		Cacheable().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			// Get current branch
			head, err := ctx.repo.Head()
			current := ""
			if err == nil && head.Name().IsBranch() {
				current = head.Name().Short()
			}

			out := branchOutput{
				Current:  current,
				Branches: make([]branchInfo, 0),
			}

			// List local branches
			branchIter, err := ctx.repo.Branches()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list branches: %w", err)
			}
			defer branchIter.Close()

			err = branchIter.ForEach(func(ref *plumbing.Reference) error {
				name := ref.Name().Short()
				out.Branches = append(out.Branches, branchInfo{
					Name:      name,
					IsCurrent: name == current,
					IsRemote:  false,
				})
				return nil
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to iterate branches: %w", err)
			}

			out.Count = len(out.Branches)

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// addInput is the input for the git_add tool.
type addInput struct {
	Paths []string `json:"paths"`
	All   bool     `json:"all,omitempty"`
}

// addOutput is the output for the git_add tool.
type addOutput struct {
	Added []string `json:"added"`
	Count int      `json:"count"`
}

func addTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_add").
		WithDescription("Stage files for commit").
		Destructive().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			var in addInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			worktree, err := ctx.repo.Worktree()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get worktree: %w", err)
			}

			out := addOutput{
				Added: make([]string, 0),
			}

			if in.All {
				// Add all changes
				_, err := worktree.Add(".")
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to add all: %w", err)
				}
				out.Added = append(out.Added, ".")
			} else {
				// Add specific paths
				for _, path := range in.Paths {
					_, err := worktree.Add(path)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to add %s: %w", path, err)
					}
					out.Added = append(out.Added, path)
				}
			}

			out.Count = len(out.Added)

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// commitInput is the input for the git_commit tool.
type commitInput struct {
	Message     string `json:"message"`
	AuthorName  string `json:"author_name,omitempty"`
	AuthorEmail string `json:"author_email,omitempty"`
}

// commitOutput is the output for the git_commit tool.
type commitOutput struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"short_hash"`
	Message   string `json:"message"`
}

func commitTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_commit").
		WithDescription("Create a commit").
		Destructive().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			var in commitInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Message == "" {
				return tool.Result{}, errors.New("commit message is required")
			}

			worktree, err := ctx.repo.Worktree()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get worktree: %w", err)
			}

			// Set author
			authorName := ctx.cfg.DefaultAuthorName
			if in.AuthorName != "" {
				authorName = in.AuthorName
			}
			authorEmail := ctx.cfg.DefaultAuthorEmail
			if in.AuthorEmail != "" {
				authorEmail = in.AuthorEmail
			}

			hash, err := worktree.Commit(in.Message, &git.CommitOptions{
				Author: &object.Signature{
					Name:  authorName,
					Email: authorEmail,
					When:  time.Now(),
				},
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to commit: %w", err)
			}

			out := commitOutput{
				Hash:      hash.String(),
				ShortHash: hash.String()[:7],
				Message:   in.Message,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// checkoutInput is the input for the git_checkout tool.
type checkoutInput struct {
	Branch string `json:"branch"`
	Create bool   `json:"create,omitempty"`
}

// checkoutOutput is the output for the git_checkout tool.
type checkoutOutput struct {
	Branch  string `json:"branch"`
	Created bool   `json:"created"`
}

func checkoutTool(ctx *packContext) tool.Tool {
	return tool.NewBuilder("git_checkout").
		WithDescription("Switch branches").
		Destructive().
		WithHandler(func(c context.Context, input json.RawMessage) (tool.Result, error) {
			var in checkoutInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Branch == "" {
				return tool.Result{}, errors.New("branch name is required")
			}

			worktree, err := ctx.repo.Worktree()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get worktree: %w", err)
			}

			opts := &git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName(in.Branch),
				Create: in.Create,
			}

			if err := worktree.Checkout(opts); err != nil {
				return tool.Result{}, fmt.Errorf("failed to checkout: %w", err)
			}

			out := checkoutOutput{
				Branch:  in.Branch,
				Created: in.Create,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
