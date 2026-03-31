// Package mcpclient provides a lightweight MCP client for health-checking
// and introspecting MCP servers over stdio or HTTP transports.
package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ctx-hq/ctx/internal/mcpproto"
)

// ConnectOptions configures how to connect to an MCP server.
type ConnectOptions struct {
	Transport string            // "stdio" or "http"
	Command   string            // (stdio) executable
	Args      []string          // (stdio) arguments
	Env       map[string]string // (stdio) extra env vars
	URL       string            // (http) endpoint URL
	Timeout   time.Duration     // overall timeout; 0 means 10s default
}

func (o *ConnectOptions) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return 10 * time.Second
}

// Client is a connected MCP client. Call Close when done.
type Client struct {
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  *bytes.Buffer
	cmd     *exec.Cmd
	httpURL string
	nextID  atomic.Int64
}

// MCPProtocolVersion is the MCP protocol version this client supports.
const MCPProtocolVersion = "2025-11-05"

// InitializeResult holds the server's initialize response.
type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
	Capabilities    map[string]any `json:"capabilities"`
}

// ServerInfo identifies the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolInfo describes a single MCP tool.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Connect establishes a connection to an MCP server.
func Connect(ctx context.Context, opts ConnectOptions) (*Client, error) {
	switch opts.Transport {
	case "http", "sse", "streamable-http":
		return connectHTTP(opts)
	default: // "stdio" or empty
		return connectStdio(ctx, opts)
	}
}

func connectStdio(ctx context.Context, opts ConnectOptions) (*Client, error) {
	if opts.Command == "" {
		return nil, &MCPError{Code: ErrProcessSpawnError, Message: "no command specified"}
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, &MCPError{Code: ErrProcessSpawnError, Message: "stdin pipe", Detail: err.Error()}
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &MCPError{Code: ErrProcessSpawnError, Message: "stdout pipe", Detail: err.Error()}
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, &MCPError{
			Code:    ErrProcessSpawnError,
			Message: fmt.Sprintf("failed to start %s", opts.Command),
			Detail:  err.Error(),
		}
	}

	return &Client{
		stdin:  stdinPipe,
		stdout: bufio.NewReader(stdoutPipe),
		stderr: &stderr,
		cmd:    cmd,
	}, nil
}

func connectHTTP(opts ConnectOptions) (*Client, error) {
	if opts.URL == "" {
		return nil, &MCPError{Code: ErrConnectionFailed, Message: "no URL specified"}
	}
	return &Client{httpURL: opts.URL}, nil
}

// Initialize sends the MCP initialize handshake.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	resp, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": MCPProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "ctx",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return nil, err
	}

	// Send initialized notification (fire-and-forget for stdio)
	if c.httpURL == "" {
		_ = c.notify("notifications/initialized", nil)
	}

	var result InitializeResult
	data, _ := json.Marshal(resp)
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &MCPError{Code: ErrProtocolError, Message: "invalid initialize result", Detail: err.Error()}
	}
	return &result, nil
}

// ListTools sends tools/list and returns available tools.
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	m, ok := resp.(map[string]any)
	if !ok {
		return nil, &MCPError{Code: ErrProtocolError, Message: "tools/list result is not an object"}
	}
	toolsRaw, ok := m["tools"]
	if !ok {
		return nil, &MCPError{Code: ErrProtocolError, Message: "tools/list missing 'tools' key"}
	}

	data, _ := json.Marshal(toolsRaw)
	var tools []ToolInfo
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, &MCPError{Code: ErrProtocolError, Message: "invalid tools format", Detail: err.Error()}
	}
	return tools, nil
}

// Close shuts down the client. For stdio, it closes stdin and waits for the process.
func (c *Client) Close() error {
	if c.cmd != nil {
		_ = c.stdin.Close()
		// Give the process a moment to exit gracefully
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = c.cmd.Process.Kill()
			<-done
		}
	}
	return nil
}

// Stderr returns captured stderr output (stdio only).
func (c *Client) Stderr() string {
	if c.stderr != nil {
		return c.stderr.String()
	}
	return ""
}

// call sends a JSON-RPC request and waits for a response.
func (c *Client) call(ctx context.Context, method string, params any) (any, error) {
	if c.httpURL != "" {
		return c.callHTTP(ctx, method, params)
	}
	return c.callStdio(ctx, method, params)
}

func (c *Client) callStdio(ctx context.Context, method string, params any) (any, error) {
	id := c.nextID.Add(1)

	var paramsRaw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, &MCPError{Code: ErrProtocolError, Message: "marshal params", Detail: err.Error()}
		}
		paramsRaw = data
	}

	req := mcpproto.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsRaw,
	}

	writer := bufio.NewWriter(c.stdin)
	if err := mcpproto.WriteMessage(writer, req); err != nil {
		return nil, &MCPError{Code: ErrConnectionFailed, Message: "write request", Detail: err.Error()}
	}

	// Read response with timeout
	type readResult struct {
		body []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		body, err := mcpproto.ReadMessage(c.stdout)
		ch <- readResult{body, err}
	}()

	select {
	case <-ctx.Done():
		return nil, &MCPError{Code: ErrInitializationTimeout, Message: "context cancelled", Detail: ctx.Err().Error()}
	case r := <-ch:
		if r.err != nil {
			detail := r.err.Error()
			if stderr := c.Stderr(); stderr != "" {
				detail += "\nstderr: " + strings.TrimSpace(stderr)
			}
			return nil, &MCPError{Code: ErrConnectionFailed, Message: "read response", Detail: detail}
		}

		var resp mcpproto.Response
		if err := json.Unmarshal(r.body, &resp); err != nil {
			return nil, &MCPError{Code: ErrProtocolError, Message: "unmarshal response", Detail: err.Error()}
		}
		if resp.Error != nil {
			detail, _ := json.Marshal(resp.Error)
			return nil, &MCPError{Code: ErrProtocolError, Message: "server error", Detail: string(detail)}
		}
		return resp.Result, nil
	}
}

func (c *Client) callHTTP(ctx context.Context, method string, params any) (any, error) {
	id := c.nextID.Add(1)

	var paramsRaw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, &MCPError{Code: ErrProtocolError, Message: "marshal params", Detail: err.Error()}
		}
		paramsRaw = data
	}

	req := mcpproto.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsRaw,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, &MCPError{Code: ErrProtocolError, Message: "marshal request", Detail: err.Error()}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.httpURL, bytes.NewReader(body))
	if err != nil {
		return nil, &MCPError{Code: ErrConnectionFailed, Message: "create HTTP request", Detail: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, &MCPError{Code: ErrConnectionFailed, Message: "HTTP request failed", Detail: err.Error()}
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &MCPError{Code: ErrConnectionFailed, Message: "read HTTP response", Detail: err.Error()}
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, &MCPError{
			Code:    ErrConnectionFailed,
			Message: fmt.Sprintf("HTTP %d", httpResp.StatusCode),
			Detail:  string(respBody),
		}
	}

	var resp mcpproto.Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &MCPError{Code: ErrProtocolError, Message: "unmarshal HTTP response", Detail: err.Error()}
	}
	if resp.Error != nil {
		detail, _ := json.Marshal(resp.Error)
		return nil, &MCPError{Code: ErrProtocolError, Message: "server error", Detail: string(detail)}
	}
	return resp.Result, nil
}

// notify sends a JSON-RPC notification (no response expected).
func (c *Client) notify(method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = data
	}

	req := mcpproto.Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsRaw,
	}

	writer := bufio.NewWriter(c.stdin)
	return mcpproto.WriteMessage(writer, req)
}
