// Package mcp provides Model Context Protocol integration for agent-go.
// It wraps github.com/felixgeelhaar/mcp-go to expose agent tools via MCP
// and consume external MCP tools.
package mcp

import (
	mcpgo "github.com/felixgeelhaar/mcp-go"
)

// Re-export core types from mcp-go for convenience.
type (
	// ServerInfo contains MCP server metadata.
	ServerInfo = mcpgo.ServerInfo

	// Capabilities declares features the server supports.
	Capabilities = mcpgo.Capabilities

	// ServeOption configures server behavior.
	ServeOption = mcpgo.ServeOption

	// HTTPOption configures HTTP transport.
	HTTPOption = mcpgo.HTTPOption

	// Middleware is a function that wraps request handling.
	Middleware = mcpgo.Middleware

	// Logger is the logging interface for MCP.
	Logger = mcpgo.Logger
)

// Re-export constructors and functions from mcp-go.
var (
	// NewServer creates a new MCP server.
	NewMCPServer = mcpgo.NewServer

	// ServeStdio runs the server over stdin/stdout.
	ServeStdioMCP = mcpgo.ServeStdio

	// ServeHTTP runs the server over HTTP with SSE.
	ServeHTTPMCP = mcpgo.ServeHTTP

	// WithMiddleware adds middleware to serve options.
	WithMiddleware = mcpgo.WithMiddleware

	// WithLogger adds a logger to serve options.
	WithLogger = mcpgo.WithLogger

	// WithInstructions sets server instructions.
	WithInstructions = mcpgo.WithInstructions

	// Middleware constructors
	Recover   = mcpgo.Recover
	RequestID = mcpgo.RequestID
	Timeout   = mcpgo.Timeout
	Logging   = mcpgo.Logging
	Chain     = mcpgo.Chain
)
