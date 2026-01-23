// Package git provides Git operation tools for agent-go.
//
// This pack includes tools for Git version control:
//   - git_status: Get repository status
//   - git_log: View commit history
//   - git_diff: Show changes between commits or working tree
//   - git_branch: List, create, or delete branches
//   - git_checkout: Switch branches or restore files
//   - git_commit: Create a new commit
//   - git_push: Push commits to remote
//   - git_pull: Pull changes from remote
//   - git_clone: Clone a repository
//   - git_add: Stage files for commit
//   - git_reset: Unstage files or reset to a commit
//
// Supports authentication via SSH keys or credentials.
package git

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the Git tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("git").
		WithDescription("Git version control tools").
		WithVersion("0.1.0").
		AddTools(
			gitStatus(),
			gitLog(),
			gitDiff(),
			gitBranch(),
			gitCheckout(),
			gitCommit(),
			gitPush(),
			gitPull(),
			gitClone(),
			gitAdd(),
			gitReset(),
		).
		AllowInState(agent.StateExplore, "git_status", "git_log", "git_diff", "git_branch").
		AllowInState(agent.StateAct, "git_status", "git_log", "git_diff", "git_branch", "git_checkout", "git_commit", "git_push", "git_pull", "git_clone", "git_add", "git_reset").
		AllowInState(agent.StateValidate, "git_status", "git_log", "git_diff").
		Build()
}

func gitStatus() tool.Tool {
	return tool.NewBuilder("git_status").
		WithDescription("Get the status of the working tree").
		ReadOnly().
		MustBuild()
}

func gitLog() tool.Tool {
	return tool.NewBuilder("git_log").
		WithDescription("View commit history").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func gitDiff() tool.Tool {
	return tool.NewBuilder("git_diff").
		WithDescription("Show changes between commits, branches, or working tree").
		ReadOnly().
		MustBuild()
}

func gitBranch() tool.Tool {
	return tool.NewBuilder("git_branch").
		WithDescription("List, create, or delete branches").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func gitCheckout() tool.Tool {
	return tool.NewBuilder("git_checkout").
		WithDescription("Switch branches or restore working tree files").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func gitCommit() tool.Tool {
	return tool.NewBuilder("git_commit").
		WithDescription("Create a new commit with staged changes").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func gitPush() tool.Tool {
	return tool.NewBuilder("git_push").
		WithDescription("Push commits to a remote repository").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func gitPull() tool.Tool {
	return tool.NewBuilder("git_pull").
		WithDescription("Pull changes from a remote repository").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func gitClone() tool.Tool {
	return tool.NewBuilder("git_clone").
		WithDescription("Clone a repository into a new directory").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func gitAdd() tool.Tool {
	return tool.NewBuilder("git_add").
		WithDescription("Stage files for the next commit").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func gitReset() tool.Tool {
	return tool.NewBuilder("git_reset").
		WithDescription("Unstage files or reset to a previous commit").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}
