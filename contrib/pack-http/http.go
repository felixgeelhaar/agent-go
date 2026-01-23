// Package http provides HTTP request tools for agent-go.
//
// This pack includes tools for making HTTP requests:
//   - http_get: Perform HTTP GET requests
//   - http_post: Perform HTTP POST requests with JSON body
//   - http_put: Perform HTTP PUT requests with JSON body
//   - http_delete: Perform HTTP DELETE requests
//   - http_head: Perform HTTP HEAD requests
//   - http_patch: Perform HTTP PATCH requests with JSON body
//
// All tools support custom headers, timeouts, and authentication.
package http

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the HTTP tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("http").
		WithDescription("HTTP request tools for making web API calls").
		WithVersion("0.1.0").
		AddTools(
			httpGet(),
			httpPost(),
			httpPut(),
			httpDelete(),
			httpHead(),
			httpPatch(),
		).
		AllowInState(agent.StateExplore, "http_get", "http_head").
		AllowInState(agent.StateAct, "http_get", "http_post", "http_put", "http_delete", "http_head", "http_patch").
		Build()
}

func httpGet() tool.Tool {
	return tool.NewBuilder("http_get").
		WithDescription("Perform an HTTP GET request").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func httpPost() tool.Tool {
	return tool.NewBuilder("http_post").
		WithDescription("Perform an HTTP POST request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func httpPut() tool.Tool {
	return tool.NewBuilder("http_put").
		WithDescription("Perform an HTTP PUT request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func httpDelete() tool.Tool {
	return tool.NewBuilder("http_delete").
		WithDescription("Perform an HTTP DELETE request").
		Destructive().
		MustBuild()
}

func httpHead() tool.Tool {
	return tool.NewBuilder("http_head").
		WithDescription("Perform an HTTP HEAD request").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func httpPatch() tool.Tool {
	return tool.NewBuilder("http_patch").
		WithDescription("Perform an HTTP PATCH request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}
