// Package copilot provides GitHub Copilot integration tools for agent-go.
//
// This pack includes tools for GitHub Copilot operations:
//   - copilot_complete: Get code completions from Copilot
//   - copilot_explain: Get explanations for code snippets
//   - copilot_suggest: Get code suggestions for a task
//   - copilot_review: Get code review suggestions
//   - copilot_test: Generate test cases for code
//   - copilot_doc: Generate documentation for code
//
// Requires GitHub Copilot authentication.
// Respects rate limits and usage quotas.
package copilot

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the GitHub Copilot tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("copilot").
		WithDescription("GitHub Copilot integration tools for code assistance").
		WithVersion("0.1.0").
		AddTools(
			copilotComplete(),
			copilotExplain(),
			copilotSuggest(),
			copilotReview(),
			copilotTest(),
			copilotDoc(),
		).
		AllowInState(agent.StateExplore, "copilot_explain", "copilot_review").
		AllowInState(agent.StateAct, "copilot_complete", "copilot_explain", "copilot_suggest", "copilot_review", "copilot_test", "copilot_doc").
		AllowInState(agent.StateDecide, "copilot_suggest").
		Build()
}

func copilotComplete() tool.Tool {
	return tool.NewBuilder("copilot_complete").
		WithDescription("Get code completions from GitHub Copilot").
		ReadOnly().
		MustBuild()
}

func copilotExplain() tool.Tool {
	return tool.NewBuilder("copilot_explain").
		WithDescription("Get explanations for code snippets").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func copilotSuggest() tool.Tool {
	return tool.NewBuilder("copilot_suggest").
		WithDescription("Get code suggestions for implementing a task").
		ReadOnly().
		MustBuild()
}

func copilotReview() tool.Tool {
	return tool.NewBuilder("copilot_review").
		WithDescription("Get code review suggestions and improvements").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func copilotTest() tool.Tool {
	return tool.NewBuilder("copilot_test").
		WithDescription("Generate test cases for code").
		ReadOnly().
		MustBuild()
}

func copilotDoc() tool.Tool {
	return tool.NewBuilder("copilot_doc").
		WithDescription("Generate documentation for code").
		ReadOnly().
		Cacheable().
		MustBuild()
}
