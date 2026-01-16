package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

var (
	// ErrNotConnected indicates the client is not connected.
	ErrNotConnected = errors.New("client not connected")

	// ErrAlreadyConnected indicates the client is already connected.
	ErrAlreadyConnected = errors.New("client already connected")

	// ErrConnectionFailed indicates the connection to the server failed.
	ErrConnectionFailed = errors.New("connection failed")
)

// ClientTransport defines how to connect to an MCP server.
type ClientTransport string

const (
	// ClientTransportStdio connects via stdin/stdout to a subprocess.
	ClientTransportStdio ClientTransport = "stdio"

	// ClientTransportSSE connects via Server-Sent Events.
	ClientTransportSSE ClientTransport = "sse"

	// ClientTransportHTTP connects via HTTP POST.
	ClientTransportHTTP ClientTransport = "http"
)

// MCPToolDef represents a tool definition from an MCP server.
type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolCall represents a tool call request.
type MCPToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// MCPToolResult represents the result of a tool call.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in an MCP response.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ClientConfig configures an MCP client.
type ClientConfig struct {
	// Name is the client name.
	Name string

	// Version is the client version.
	Version string

	// Command is the server command to run (for stdio transport).
	Command []string

	// Transport specifies the communication transport.
	Transport ClientTransport

	// URL is the URL for SSE/HTTP transport.
	URL string
}

// ClientOption configures a client.
type ClientOption func(*ClientConfig)

// WithClientName sets the client name.
func WithClientName(name string) ClientOption {
	return func(c *ClientConfig) {
		c.Name = name
	}
}

// WithClientVersion sets the client version.
func WithClientVersion(version string) ClientOption {
	return func(c *ClientConfig) {
		c.Version = version
	}
}

// WithServerCommand sets the server command for stdio transport.
func WithServerCommand(cmd ...string) ClientOption {
	return func(c *ClientConfig) {
		c.Command = cmd
		c.Transport = ClientTransportStdio
	}
}

// WithSSEURL sets the URL for SSE transport.
func WithSSEURL(url string) ClientOption {
	return func(c *ClientConfig) {
		c.URL = url
		c.Transport = ClientTransportSSE
	}
}

// WithHTTPURL sets the URL for HTTP transport.
func WithHTTPURL(url string) ClientOption {
	return func(c *ClientConfig) {
		c.URL = url
		c.Transport = ClientTransportHTTP
	}
}

// MCPClient consumes tools from an MCP server.
type MCPClient struct {
	config     ClientConfig
	serverInfo *MCPServerInfo
	connected  bool
	mu         sync.RWMutex

	// For stdio transport
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
	encoder *json.Encoder

	// Request tracking
	reqID     atomic.Int64
	responses map[int64]chan *rpcResponse
	respMu    sync.Mutex
}

// MCPServerInfo contains information about an MCP server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// JSON-RPC types for MCP communication.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      interface{}  `json:"id,omitempty"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *rpcError    `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initParams struct {
	ProtocolVersion string        `json:"protocolVersion"`
	Capabilities    interface{}   `json:"capabilities"`
	ClientInfo      MCPServerInfo `json:"clientInfo"`
}

type initResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	ServerInfo      MCPServerInfo `json:"serverInfo"`
}

type listToolsRes struct {
	Tools []MCPToolDef `json:"tools"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// NewClient creates a new MCP client.
func NewClient(opts ...ClientOption) *MCPClient {
	cfg := ClientConfig{
		Name:      "agent-go-client",
		Version:   "1.0.0",
		Transport: ClientTransportStdio,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &MCPClient{
		config:    cfg,
		responses: make(map[int64]chan *rpcResponse),
	}
}

// Connect connects to an MCP server.
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return ErrAlreadyConnected
	}

	switch c.config.Transport {
	case ClientTransportStdio:
		if err := c.connectStdio(ctx); err != nil {
			return err
		}
	case ClientTransportSSE, ClientTransportHTTP:
		return fmt.Errorf("transport %s not yet implemented", c.config.Transport)
	default:
		return fmt.Errorf("unknown transport: %s", c.config.Transport)
	}

	c.connected = true
	return nil
}

func (c *MCPClient) connectStdio(ctx context.Context) error {
	if len(c.config.Command) == 0 {
		return fmt.Errorf("%w: no command specified", ErrConnectionFailed)
	}

	// Start the server process
	c.cmd = exec.CommandContext(ctx, c.config.Command[0], c.config.Command[1:]...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%w: stdin pipe: %v", ErrConnectionFailed, err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		_ = c.stdin.Close()
		return fmt.Errorf("%w: stdout pipe: %v", ErrConnectionFailed, err)
	}

	if err := c.cmd.Start(); err != nil {
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		return fmt.Errorf("%w: start command: %v", ErrConnectionFailed, err)
	}

	c.scanner = bufio.NewScanner(c.stdout)
	c.encoder = json.NewEncoder(c.stdin)

	// Start response reader goroutine
	go c.readResponses()

	// Initialize the connection
	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		return err
	}

	return nil
}

func (c *MCPClient) readResponses() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		// Get the request ID
		var reqID int64
		switch id := resp.ID.(type) {
		case float64:
			reqID = int64(id)
		case int64:
			reqID = id
		case int:
			reqID = int64(id)
		default:
			continue
		}

		// Send to waiting goroutine
		c.respMu.Lock()
		if ch, exists := c.responses[reqID]; exists {
			ch <- &resp
			delete(c.responses, reqID)
		}
		c.respMu.Unlock()
	}
}

func (c *MCPClient) initialize(ctx context.Context) error {
	params := initParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: MCPServerInfo{
			Name:    c.config.Name,
			Version: c.config.Version,
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Parse the result
	var result initResult
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	c.serverInfo = &result.ServerInfo

	// Send initialized notification
	notification := rpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	return c.encoder.Encode(notification)
}

func (c *MCPClient) sendRequest(ctx context.Context, method string, params interface{}) (*rpcResponse, error) {
	id := c.reqID.Add(1)

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsBytes,
	}

	// Create response channel
	respCh := make(chan *rpcResponse, 1)
	c.respMu.Lock()
	c.responses[id] = respCh
	c.respMu.Unlock()

	// Send the request
	if err := c.encoder.Encode(req); err != nil {
		c.respMu.Lock()
		delete(c.responses, id)
		c.respMu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		c.respMu.Lock()
		delete(c.responses, id)
		c.respMu.Unlock()
		return nil, ctx.Err()
	}
}

// Close closes the connection to the server.
func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false

	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}

	return nil
}

// ListTools returns available tools from the server.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return nil, ErrNotConnected
	}

	resp, err := c.sendRequest(ctx, "tools/list", struct{}{})
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	var result listToolsRes
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("parse list tools result: %w", err)
	}

	return result.Tools, nil
}

// CallTool calls a tool on the server.
func (c *MCPClient) CallTool(ctx context.Context, req MCPToolCall) (*MCPToolResult, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return nil, ErrNotConnected
	}

	params := callToolParams{
		Name:      req.Name,
		Arguments: req.Arguments,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("call tool error: %s", resp.Error.Message)
	}

	var result MCPToolResult
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("parse call tool result: %w", err)
	}

	return &result, nil
}

// ServerInfo returns information about the connected server.
func (c *MCPClient) ServerInfo() *MCPServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// Tools returns all server tools as agent-go tools.
// This allows MCP tools to be added to an agent's tool registry.
func (c *MCPClient) Tools(ctx context.Context) ([]tool.Tool, error) {
	defs, err := c.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	tools := make([]tool.Tool, len(defs))
	for i, def := range defs {
		tools[i] = newMCPProxyTool(def, c.createToolCaller())
	}

	return tools, nil
}

func (c *MCPClient) createToolCaller() func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
	return func(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
		result, err := c.CallTool(ctx, MCPToolCall{
			Name:      name,
			Arguments: input,
		})
		if err != nil {
			return tool.Result{}, err
		}

		if result.IsError {
			if len(result.Content) > 0 {
				return tool.Result{}, errors.New(result.Content[0].Text)
			}
			return tool.Result{}, errors.New("tool execution failed")
		}

		// Convert content to JSON output
		if len(result.Content) > 0 {
			output, _ := json.Marshal(map[string]string{
				"content": result.Content[0].Text,
			})
			return tool.Result{Output: output}, nil
		}

		return tool.Result{Output: json.RawMessage(`{}`)}, nil
	}
}

// ImportToolsFromClient imports all MCP tools from a client into a tool registry.
func ImportToolsFromClient(ctx context.Context, client *MCPClient, registry tool.Registry) error {
	tools, err := client.Tools(ctx)
	if err != nil {
		return err
	}

	for _, t := range tools {
		if err := registry.Register(t); err != nil {
			// Continue on duplicate registration
			continue
		}
	}

	return nil
}
