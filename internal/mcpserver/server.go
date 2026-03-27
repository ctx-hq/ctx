package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Server implements a minimal MCP server over stdio.
type Server struct {
	tools map[string]ToolHandler
}

// ToolHandler handles an MCP tool call.
type ToolHandler func(args json.RawMessage) (any, error)

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
func (s *Server) Serve() error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for {
		// Read Content-Length header
		contentLen, err := readContentLength(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read header: %w", err)
		}

		// Read exactly contentLen bytes of JSON body
		body := make([]byte, contentLen)
		if _, err := io.ReadFull(reader, body); err != nil {
			writeFramedError(writer, nil, -32700, "Failed to read message body")
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeFramedError(writer, nil, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(&req)
		if resp == nil {
			continue // notifications don't produce a response
		}

		data, _ := json.Marshal(resp)
		writeFrame(writer, data)
	}
}

// readContentLength reads the "Content-Length: N\r\n\r\n" header from the reader.
func readContentLength(r *bufio.Reader) (int, error) {
	// Read lines until we find Content-Length, then consume the blank line separator
	var contentLen int
	foundHeader := false

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line = end of headers
			if !foundHeader {
				continue // skip leading blank lines
			}
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			n, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			contentLen = n
			foundHeader = true
		}
	}

	if !foundHeader {
		return 0, fmt.Errorf("missing Content-Length header")
	}
	return contentLen, nil
}

// writeFrame writes a JSON-RPC message with Content-Length framing.
func writeFrame(w *bufio.Writer, data []byte) {
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data))
	w.Write(data)
	w.Flush()
}

// writeFramedError writes a JSON-RPC error response with Content-Length framing.
func writeFramedError(w *bufio.Writer, id any, code int, message string) {
	resp := errorResponse(id, code, message)
	data, _ := json.Marshal(resp)
	writeFrame(w, data)
}

func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
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
		return &JSONRPCResponse{
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
			return errorResponse(req.ID, -32602, "Invalid params")
		}

		handler, ok := s.tools[params.Name]
		if !ok {
			return errorResponse(req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
		}

		result, err := handler(params.Arguments)
		if err != nil {
			return &JSONRPCResponse{
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
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}

	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func errorResponse(id any, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   map[string]any{"code": code, "message": message},
	}
}

