package mcpclient

import "fmt"

// ErrorCode classifies MCP connection/test failures.
type ErrorCode string

const (
	ErrConnectionFailed      ErrorCode = "CONNECTION_FAILED"
	ErrProcessSpawnError     ErrorCode = "PROCESS_SPAWN_ERROR"
	ErrInitializationTimeout ErrorCode = "INITIALIZATION_TIMEOUT"
	ErrValidationError       ErrorCode = "VALIDATION_ERROR"
	ErrProtocolError         ErrorCode = "PROTOCOL_ERROR"
)

// MCPError is a classified MCP client error with diagnostic detail.
type MCPError struct {
	Code    ErrorCode
	Message string
	Detail  string // stderr output, HTTP status, exit code, etc.
}

func (e *MCPError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
