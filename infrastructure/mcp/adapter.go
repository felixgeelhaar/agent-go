package mcp

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// mcpProxyTool wraps an MCP tool as an agent-go tool.
type mcpProxyTool struct {
	def    MCPToolDef
	caller func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error)
	annot  tool.Annotations
}

// newMCPProxyTool creates a new MCP proxy tool.
func newMCPProxyTool(def MCPToolDef, caller func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error)) *mcpProxyTool {
	return &mcpProxyTool{
		def:    def,
		caller: caller,
	}
}

func (t *mcpProxyTool) Name() string {
	return t.def.Name
}

func (t *mcpProxyTool) Description() string {
	return t.def.Description
}

func (t *mcpProxyTool) InputSchema() tool.Schema {
	if len(t.def.InputSchema) == 0 {
		return tool.EmptySchema()
	}
	return tool.NewSchema(t.def.InputSchema)
}

func (t *mcpProxyTool) OutputSchema() tool.Schema {
	return tool.EmptySchema()
}

func (t *mcpProxyTool) Annotations() tool.Annotations {
	return t.annot
}

func (t *mcpProxyTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	return t.caller(ctx, t.def.Name, input)
}

// ToolToMCPDef converts an agent-go tool to an MCP tool definition.
func ToolToMCPDef(t tool.Tool) MCPToolDef {
	def := MCPToolDef{
		Name:        t.Name(),
		Description: t.Description(),
	}

	// Get the raw JSON schema
	schema := t.InputSchema()
	if !schema.IsEmpty() {
		def.InputSchema = schema.Raw()
	}

	return def
}

// MCPDefToTool converts an MCP tool definition to an agent-go tool.
// The caller function is used to execute the tool on the remote server.
func MCPDefToTool(def MCPToolDef, caller func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error)) tool.Tool {
	return newMCPProxyTool(def, caller)
}
