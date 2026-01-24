module github.com/felixgeelhaar/agent-go/contrib/pack-websocket

go 1.25.0

require (
	github.com/felixgeelhaar/agent-go v0.0.0
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
)

require golang.org/x/net v0.49.0 // indirect

replace github.com/felixgeelhaar/agent-go => ../..
