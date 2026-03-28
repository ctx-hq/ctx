package output

import (
	"errors"
	"fmt"
	"strings"
)

// Error codes — machine-readable identifiers for error categories.
const (
	CodeUsage     = "usage"
	CodeNotFound  = "not_found"
	CodeAuth      = "auth"
	CodeForbidden = "forbidden"
	CodeRateLimit = "rate_limit"
	CodeNetwork   = "network"
	CodeAPI       = "api"
	CodeAmbiguous = "ambiguous"
)

// Exit codes — matching basecamp/fizzy/hey industry standard.
const (
	ExitUsage     = 1
	ExitNotFound  = 2
	ExitAuth      = 3
	ExitForbidden = 4
	ExitRateLimit = 5
	ExitNetwork   = 6
	ExitAPI       = 7
	ExitAmbiguous = 8
)

// CLIError is a typed error with structured metadata for both human and machine consumption.
type CLIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Hint       string `json:"hint,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
	Cause      error  `json:"-"`
}

func (e *CLIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n  Hint: %s", e.Message, e.Hint)
	}
	return e.Message
}

func (e *CLIError) Unwrap() error {
	return e.Cause
}

// ExitCode returns the process exit code for this error.
func (e *CLIError) ExitCode() int {
	switch e.Code {
	case CodeUsage:
		return ExitUsage
	case CodeNotFound:
		return ExitNotFound
	case CodeAuth:
		return ExitAuth
	case CodeForbidden:
		return ExitForbidden
	case CodeRateLimit:
		return ExitRateLimit
	case CodeNetwork:
		return ExitNetwork
	case CodeAPI:
		return ExitAPI
	case CodeAmbiguous:
		return ExitAmbiguous
	default:
		return ExitUsage
	}
}

// AsCLIError extracts a CLIError from an error chain, or returns nil.
func AsCLIError(err error) *CLIError {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr
	}
	return nil
}

// --- Constructors ---

// ErrUsage creates an error for invalid arguments or usage.
func ErrUsage(msg string) *CLIError {
	return &CLIError{Code: CodeUsage, Message: msg}
}

// ErrUsageHint creates a usage error with an actionable hint.
func ErrUsageHint(msg, hint string) *CLIError {
	return &CLIError{Code: CodeUsage, Message: msg, Hint: hint}
}

// ErrNotFound creates a 404-style error for missing resources.
func ErrNotFound(resource, identifier string) *CLIError {
	return &CLIError{
		Code:       CodeNotFound,
		Message:    fmt.Sprintf("%s %q not found", resource, identifier),
		HTTPStatus: 404,
	}
}

// ErrAuth creates an authentication error with login hint.
func ErrAuth(msg string) *CLIError {
	return &CLIError{
		Code:       CodeAuth,
		Message:    msg,
		Hint:       "Run 'ctx login' to authenticate",
		HTTPStatus: 401,
	}
}

// ErrForbidden creates a permission denied error.
func ErrForbidden(msg string) *CLIError {
	return &CLIError{
		Code:       CodeForbidden,
		Message:    msg,
		HTTPStatus: 403,
	}
}

// ErrRateLimit creates a rate limit error with optional retry-after info.
func ErrRateLimit(retryAfter int) *CLIError {
	msg := "rate limited by the registry"
	hint := "Please wait a moment and try again"
	if retryAfter > 0 {
		hint = fmt.Sprintf("Retry after %d seconds", retryAfter)
	}
	return &CLIError{
		Code:       CodeRateLimit,
		Message:    msg,
		Hint:       hint,
		HTTPStatus: 429,
		Retryable:  true,
	}
}

// ErrNetwork creates a network connectivity error.
func ErrNetwork(cause error) *CLIError {
	msg := "network error"
	if cause != nil {
		msg = fmt.Sprintf("network error: %v", cause)
	}
	return &CLIError{
		Code:      CodeNetwork,
		Message:   msg,
		Hint:      "Check your internet connection and try again",
		Retryable: true,
		Cause:     cause,
	}
}

// ErrAPI creates a general API error.
func ErrAPI(status int, msg string) *CLIError {
	return &CLIError{
		Code:       CodeAPI,
		Message:    msg,
		HTTPStatus: status,
		Retryable:  status >= 500,
	}
}

// ErrAmbiguous creates an error for ambiguous resource matches.
func ErrAmbiguous(resource string, matches []string) *CLIError {
	return &CLIError{
		Code:    CodeAmbiguous,
		Message: fmt.Sprintf("ambiguous %s, multiple matches: %s", resource, strings.Join(matches, ", ")),
		Hint:    "Please be more specific",
	}
}

// FromHTTPStatus creates a CLIError from an HTTP status code and message.
func FromHTTPStatus(status int, msg string) *CLIError {
	switch {
	case status == 401:
		return ErrAuth(msg)
	case status == 403:
		return ErrForbidden(msg)
	case status == 404:
		return &CLIError{Code: CodeNotFound, Message: msg, HTTPStatus: 404}
	case status == 422:
		return &CLIError{Code: CodeUsage, Message: msg, HTTPStatus: 422}
	case status == 429:
		return ErrRateLimit(0)
	case status >= 500:
		return &CLIError{
			Code:       CodeAPI,
			Message:    fmt.Sprintf("server error (%d): %s", status, msg),
			Hint:       "The server returned a temporary error. Try again in a moment.",
			HTTPStatus: status,
			Retryable:  true,
		}
	default:
		return ErrAPI(status, msg)
	}
}
