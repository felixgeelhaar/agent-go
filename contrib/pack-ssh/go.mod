module github.com/felixgeelhaar/agent-go/contrib/pack-ssh

go 1.25.0

require (
	github.com/felixgeelhaar/agent-go v0.0.0
	golang.org/x/crypto v0.47.0
)

require golang.org/x/sys v0.40.0 // indirect

replace github.com/felixgeelhaar/agent-go => ../..
