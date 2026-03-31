package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ctx-hq/ctx/internal/mcpproto"
)

// Server implements a minimal MCP server over stdio.
type Server struct {
	tools map[string]ToolHandler
}

// ToolHandler handles an MCP tool call.
type ToolHandler func(ctx context.Context, args json.RawMessage) (any, error)

// New creates a new MCP server.
func New() *Server {
	s := &Server{
		tools: make(map[string]ToolHandler),
	}
	RegisterDefaultTools(s)
	return s
}

// RegisterTool adds a tool to the server.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// Serve runs the MCP server over stdio using Content-Length framing (MCP spec).
func (s *Server) Serve(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for {
		body, err := mcpproto.ReadMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var req mcpproto.Request
		if err := json.Unmarshal(body, &req); err != nil {
			if err := mcpproto.WriteFramedError(writer, nil, -32700, "Parse error"); err != nil {
				return fmt.Errorf("write error frame: %w", err)
			}
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp == nil {
			continue // notifications don't produce a response
		}

		if err := mcpproto.WriteMessage(writer, resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req *mcpproto.Request) *mcpproto.Response {
	switch req.Method {
	case "initialize":
		return &mcpproto.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2025-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "ctx",
					"version": "0.1.0",
				},
			},
		}

	case "notifications/initialized":
		return nil // notification, no response

	case "tools/list":
		tools := make([]map[string]any, 0)
		for name := range s.tools {
			if def, ok := toolDefinitions[name]; ok {
				tools = append(tools, def)
			}
		}
		return &mcpproto.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpproto.ErrorResponse(req.ID, -32602, "Invalid params")
		}

		handler, ok := s.tools[params.Name]
		if !ok {
			return mcpproto.ErrorResponse(req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
		}

		result, err := handler(ctx, params.Arguments)
		if err != nil {
			return &mcpproto.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
					},
					"isError": true,
				},
			}
		}

		text, _ := json.MarshalIndent(result, "", "  ")
		return &mcpproto.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}

	default:
		return mcpproto.ErrorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}
