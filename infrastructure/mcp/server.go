package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	mcpgo "github.com/felixgeelhaar/mcp-go"
	mcpserver "github.com/felixgeelhaar/mcp-go/server"
)

// AgentServer wraps an MCP server to expose agent-go tools.
type AgentServer struct {
	srv      *mcpgo.Server
	registry tool.Registry
	info     mcpgo.ServerInfo
}

// AgentServerConfig configures an agent MCP server.
type AgentServerConfig struct {
	// Name is the server name.
	Name string

	// Version is the server version.
	Version string

	// Registry is the tool registry containing tools to expose.
	Registry tool.Registry

	// Description is an optional server description.
	Description string

	// Instructions provides usage instructions for clients.
	Instructions string
}

// NewAgentServer creates a new MCP server that exposes agent-go tools.
func NewAgentServer(cfg AgentServerConfig) *AgentServer {
	info := mcpgo.ServerInfo{
		Name:         cfg.Name,
		Version:      cfg.Version,
		Description:  cfg.Description,
		Capabilities: mcpgo.Capabilities{
			Tools: true,
		},
	}

	// Build server options
	var opts []mcpgo.Option
	if cfg.Instructions != "" {
		opts = append(opts, mcpgo.WithInstructions(cfg.Instructions))
	}

	srv := mcpgo.NewServer(info, opts...)

	as := &AgentServer{
		srv:      srv,
		registry: cfg.Registry,
		info:     info,
	}

	// Register all tools from the registry
	if cfg.Registry != nil {
		as.registerTools()
	}

	return as
}

// registerTools registers all tools from the registry with the MCP server.
func (s *AgentServer) registerTools() {
	for _, t := range s.registry.List() {
		s.registerTool(t)
	}
}

// registerTool registers a single tool with the MCP server.
func (s *AgentServer) registerTool(t tool.Tool) {
	// Create a handler that wraps the agent-go tool
	handler := func(ctx context.Context, input json.RawMessage) (string, error) {
		result, err := t.Execute(ctx, input)
		if err != nil {
			return "", err
		}
		return string(result.Output), nil
	}

	// Register with mcp-go using the fluent API
	s.srv.Tool(t.Name()).
		Description(t.Description()).
		Handler(handler)
}

// Server returns the underlying mcp-go server.
func (s *AgentServer) Server() *mcpgo.Server {
	return s.srv
}

// Use adds middleware to the server.
func (s *AgentServer) Use(middlewares ...mcpserver.Middleware) {
	s.srv.Use(middlewares...)
}

// ServeStdio runs the server over stdin/stdout.
func (s *AgentServer) ServeStdio(ctx context.Context, opts ...mcpgo.ServeOption) error {
	return mcpgo.ServeStdio(ctx, s.srv, opts...)
}

// ServeHTTP runs the server over HTTP with SSE.
func (s *AgentServer) ServeHTTP(ctx context.Context, addr string, opts ...mcpgo.HTTPOption) error {
	return mcpgo.ServeHTTP(ctx, s.srv, addr, opts...)
}

// AddTool adds a tool to the server dynamically.
func (s *AgentServer) AddTool(t tool.Tool) error {
	if s.registry != nil {
		if err := s.registry.Register(t); err != nil {
			return fmt.Errorf("register tool: %w", err)
		}
	}
	s.registerTool(t)
	return nil
}

// QuickServe is a convenience function to create and run an MCP server over stdio.
func QuickServe(ctx context.Context, name, version string, registry tool.Registry) error {
	srv := NewAgentServer(AgentServerConfig{
		Name:     name,
		Version:  version,
		Registry: registry,
	})
	return srv.ServeStdio(ctx)
}

// Example usage:
//
//	registry := memory.NewToolRegistry()
//	registry.Register(myTool)
//
//	srv := mcp.NewAgentServer(mcp.AgentServerConfig{
//	    Name:     "my-agent",
//	    Version:  "1.0.0",
//	    Registry: registry,
//	})
//
//	// Add middleware
//	srv.Use(mcpgo.Recover(), mcpgo.RequestID())
//
//	// Serve over stdio
//	srv.ServeStdio(context.Background())
