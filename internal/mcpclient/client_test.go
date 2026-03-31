package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/ctx-hq/ctx/internal/mcpproto"
)

// mockServer runs a minimal MCP server over r (read) / w (write) pipes,
// responding to initialize and tools/list.
func mockServer(r io.Reader, w io.WriteCloser, tools []map[string]any) {
	defer func() { _ = w.Close() }()
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	for {
		body, err := mcpproto.ReadMessage(reader)
		if err != nil {
			return
		}

		var req mcpproto.Request
		if err := json.Unmarshal(body, &req); err != nil {
			return
		}

		// Skip notifications
		if req.ID == nil {
			continue
		}

		var resp *mcpproto.Response
		switch req.Method {
		case "initialize":
			resp = &mcpproto.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"protocolVersion": "2025-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "test-server", "version": "1.0.0"},
				},
			}
		case "tools/list":
			resp = &mcpproto.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{"tools": tools},
			}
		default:
			resp = mcpproto.ErrorResponse(req.ID, -32601, "unknown method")
		}

		if err := mcpproto.WriteMessage(writer, resp); err != nil {
			return
		}
	}
}

// newPipeClient creates a Client connected to a mock server via os.Pipe,
// returning the client and a cleanup function.
func newPipeClient(t *testing.T, tools []map[string]any) *Client {
	t.Helper()

	// client writes to serverIn, server reads from serverIn
	serverInReader, clientWriter := io.Pipe()
	// server writes to clientIn, client reads from clientIn
	clientReader, serverWriter := io.Pipe()

	go mockServer(serverInReader, serverWriter, tools)

	return &Client{
		stdin:  clientWriter,
		stdout: bufio.NewReader(clientReader),
	}
}

func TestInitialize(t *testing.T) {
	client := newPipeClient(t, nil)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, "test-server")
	}
	if result.ProtocolVersion != "2025-11-05" {
		t.Errorf("ProtocolVersion = %q, want %q", result.ProtocolVersion, "2025-11-05")
	}
}

func TestListTools(t *testing.T) {
	mockTools := []map[string]any{
		{"name": "tool_a", "description": "Tool A"},
		{"name": "tool_b", "description": "Tool B"},
	}
	client := newPipeClient(t, mockTools)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Must initialize first
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	if tools[0].Name != "tool_a" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "tool_a")
	}
}

func TestRunTest_Pass(t *testing.T) {
	mockTools := []map[string]any{
		{"name": "search", "description": "Search"},
		{"name": "create", "description": "Create"},
	}
	client := newPipeClient(t, mockTools)

	// We can't use RunTest directly (it calls Connect which spawns a process),
	// so test the underlying functions instead.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	initResult, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("unexpected server name: %s", initResult.ServerInfo.Name)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}

	// Test validation
	step := validateTools(tools, []string{"search", "create"})
	if step.Status != "pass" {
		t.Errorf("validate status = %q, want pass; detail: %s", step.Status, step.Detail)
	}

	_ = client.Close()
}

func TestValidateTools_Mismatch(t *testing.T) {
	tools := []ToolInfo{
		{Name: "search"},
		{Name: "delete"},
	}

	step := validateTools(tools, []string{"search", "create"})
	if step.Status != "fail" {
		t.Errorf("validate status = %q, want fail", step.Status)
	}
	// Should mention missing "create" and undeclared "delete"
	if step.Detail == "" {
		t.Error("expected non-empty detail")
	}
}

func TestValidateTools_ExtraOnly(t *testing.T) {
	tools := []ToolInfo{
		{Name: "search"},
		{Name: "create"},
		{Name: "bonus"},
	}

	step := validateTools(tools, []string{"search", "create"})
	if step.Status != "fail" {
		t.Errorf("validate status = %q, want fail", step.Status)
	}
}

func TestMCPError_Format(t *testing.T) {
	e := &MCPError{Code: ErrConnectionFailed, Message: "timeout", Detail: "after 10s"}
	got := e.Error()
	want := "CONNECTION_FAILED: timeout (after 10s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	e2 := &MCPError{Code: ErrProcessSpawnError, Message: "not found"}
	got2 := e2.Error()
	want2 := "PROCESS_SPAWN_ERROR: not found"
	if got2 != want2 {
		t.Errorf("Error() = %q, want %q", got2, want2)
	}
}
