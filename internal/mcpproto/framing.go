// Package mcpproto provides shared MCP protocol primitives (JSON-RPC framing,
// message types) used by both the MCP server and client packages.
package mcpproto

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// --- JSON-RPC types ---

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// ErrorResponse creates a JSON-RPC error response.
func ErrorResponse(id any, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   map[string]any{"code": code, "message": message},
	}
}

// --- Content-Length framing ---

// maxFrameSize is the upper bound for a single JSON-RPC message (100 MB).
const maxFrameSize = 100 * 1024 * 1024

// maxHeaderLines caps how many lines we read while looking for Content-Length.
const maxHeaderLines = 100

// ReadContentLength reads a "Content-Length: N\r\n\r\n" header from the reader
// and returns N.
func ReadContentLength(r *bufio.Reader) (int, error) {
	var contentLen int
	foundHeader := false
	linesRead := 0

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		linesRead++
		if linesRead > maxHeaderLines {
			return 0, fmt.Errorf("too many header lines without Content-Length")
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
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
			if n <= 0 || n > maxFrameSize {
				return 0, fmt.Errorf("Content-Length %d out of range (1..%d)", n, maxFrameSize)
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

// WriteFrame writes a JSON-RPC message with Content-Length framing.
func WriteFrame(w *bufio.Writer, data []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}

// WriteFramedError writes a JSON-RPC error response with Content-Length framing.
func WriteFramedError(w *bufio.Writer, id any, code int, message string) error {
	resp := ErrorResponse(id, code, message)
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteFrame(w, data)
}

// ReadMessage reads a single Content-Length-framed JSON-RPC message from the reader.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	contentLen, err := ReadContentLength(r)
	if err != nil {
		return nil, err
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// WriteMessage marshals v to JSON and writes it as a Content-Length-framed message.
func WriteMessage(w *bufio.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return WriteFrame(w, data)
}
