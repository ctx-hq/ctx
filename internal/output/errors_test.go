package output

import (
	"errors"
	"fmt"
	"testing"
)

func TestCLIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *CLIError
		want string
	}{
		{
			name: "message only",
			err:  &CLIError{Code: CodeUsage, Message: "bad argument"},
			want: "bad argument",
		},
		{
			name: "message with hint",
			err:  &CLIError{Code: CodeAuth, Message: "not authenticated", Hint: "Run 'ctx login'"},
			want: "not authenticated\n  Hint: Run 'ctx login'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCLIError_ExitCode(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{CodeUsage, ExitUsage},
		{CodeNotFound, ExitNotFound},
		{CodeAuth, ExitAuth},
		{CodeForbidden, ExitForbidden},
		{CodeRateLimit, ExitRateLimit},
		{CodeNetwork, ExitNetwork},
		{CodeAPI, ExitAPI},
		{CodeAmbiguous, ExitAmbiguous},
		{"unknown_code", ExitUsage}, // fallback
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			e := &CLIError{Code: tt.code, Message: "test"}
			if got := e.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCLIError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	e := ErrNetwork(cause)
	if !errors.Is(e, cause) {
		t.Error("Unwrap should return the cause error")
	}

	// nil cause
	e2 := ErrUsage("no cause")
	if e2.Unwrap() != nil {
		t.Error("Unwrap should return nil when no cause")
	}
}

func TestAsCLIError(t *testing.T) {
	cliErr := ErrAuth("not logged in")

	// Direct CLIError
	if got := AsCLIError(cliErr); got != cliErr {
		t.Error("AsCLIError should extract CLIError directly")
	}

	// Wrapped CLIError
	wrapped := fmt.Errorf("wrapper: %w", cliErr)
	if got := AsCLIError(wrapped); got != cliErr {
		t.Error("AsCLIError should extract CLIError from wrapped error")
	}

	// Non-CLIError
	if got := AsCLIError(fmt.Errorf("plain error")); got != nil {
		t.Error("AsCLIError should return nil for non-CLIError")
	}

	// Nil error
	if got := AsCLIError(nil); got != nil {
		t.Error("AsCLIError should return nil for nil error")
	}
}

func TestConstructors(t *testing.T) {
	t.Run("ErrUsage", func(t *testing.T) {
		e := ErrUsage("bad args")
		assertError(t, e, CodeUsage, ExitUsage, false)
	})

	t.Run("ErrUsageHint", func(t *testing.T) {
		e := ErrUsageHint("bad args", "try --help")
		assertError(t, e, CodeUsage, ExitUsage, false)
		if e.Hint != "try --help" {
			t.Errorf("Hint = %q, want %q", e.Hint, "try --help")
		}
	})

	t.Run("ErrNotFound", func(t *testing.T) {
		e := ErrNotFound("package", "@hong/review")
		assertError(t, e, CodeNotFound, ExitNotFound, false)
		if e.HTTPStatus != 404 {
			t.Errorf("HTTPStatus = %d, want 404", e.HTTPStatus)
		}
	})

	t.Run("ErrAuth", func(t *testing.T) {
		e := ErrAuth("token expired")
		assertError(t, e, CodeAuth, ExitAuth, false)
		if e.Hint == "" {
			t.Error("ErrAuth should have a hint")
		}
	})

	t.Run("ErrForbidden", func(t *testing.T) {
		e := ErrForbidden("access denied")
		assertError(t, e, CodeForbidden, ExitForbidden, false)
	})

	t.Run("ErrRateLimit_with_retry", func(t *testing.T) {
		e := ErrRateLimit(30)
		assertError(t, e, CodeRateLimit, ExitRateLimit, true)
		if e.HTTPStatus != 429 {
			t.Errorf("HTTPStatus = %d, want 429", e.HTTPStatus)
		}
	})

	t.Run("ErrRateLimit_no_retry", func(t *testing.T) {
		e := ErrRateLimit(0)
		assertError(t, e, CodeRateLimit, ExitRateLimit, true)
	})

	t.Run("ErrNetwork", func(t *testing.T) {
		cause := fmt.Errorf("connection refused")
		e := ErrNetwork(cause)
		assertError(t, e, CodeNetwork, ExitNetwork, true)
		if e.Cause != cause {
			t.Error("ErrNetwork should preserve cause")
		}
	})

	t.Run("ErrNetwork_nil_cause", func(t *testing.T) {
		e := ErrNetwork(nil)
		assertError(t, e, CodeNetwork, ExitNetwork, true)
	})

	t.Run("ErrAPI", func(t *testing.T) {
		e := ErrAPI(500, "internal server error")
		assertError(t, e, CodeAPI, ExitAPI, true)

		e2 := ErrAPI(400, "bad request")
		assertError(t, e2, CodeAPI, ExitAPI, false)
	})

	t.Run("ErrAmbiguous", func(t *testing.T) {
		e := ErrAmbiguous("agent", []string{"claude", "claude-code"})
		assertError(t, e, CodeAmbiguous, ExitAmbiguous, false)
	})
}

func TestFromHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		wantCode string
	}{
		{401, CodeAuth},
		{403, CodeForbidden},
		{404, CodeNotFound},
		{422, CodeUsage},
		{429, CodeRateLimit},
		{500, CodeAPI},
		{502, CodeAPI},
		{503, CodeAPI},
		{400, CodeAPI},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			e := FromHTTPStatus(tt.status, "test error")
			if e.Code != tt.wantCode {
				t.Errorf("FromHTTPStatus(%d).Code = %q, want %q", tt.status, e.Code, tt.wantCode)
			}
			if tt.status >= 500 && !e.Retryable {
				t.Errorf("FromHTTPStatus(%d) should be retryable", tt.status)
			}
		})
	}
}

func assertError(t *testing.T, e *CLIError, wantCode string, wantExit int, wantRetryable bool) {
	t.Helper()
	if e.Code != wantCode {
		t.Errorf("Code = %q, want %q", e.Code, wantCode)
	}
	if e.ExitCode() != wantExit {
		t.Errorf("ExitCode() = %d, want %d", e.ExitCode(), wantExit)
	}
	if e.Retryable != wantRetryable {
		t.Errorf("Retryable = %v, want %v", e.Retryable, wantRetryable)
	}
	if e.Message == "" {
		t.Error("Message should not be empty")
	}
}
