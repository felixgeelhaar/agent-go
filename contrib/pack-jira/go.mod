module github.com/felixgeelhaar/agent-go/contrib/pack-jira

go 1.25.0

require (
	github.com/felixgeelhaar/agent-go v0.0.0
	github.com/felixgeelhaar/jirasdk v1.0.0
)

require golang.org/x/oauth2 v0.34.0 // indirect

replace github.com/felixgeelhaar/agent-go => ../..
