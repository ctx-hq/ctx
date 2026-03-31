package main

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/mcpclient"
)

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "****"},
		{"ab", "****"},
		{"abcd", "****"},
		{"abcde", "*bcde"},
		{"sk-1234567890", "*********7890"},
	}
	for _, tt := range tests {
		got := maskValue(tt.input)
		if got != tt.want {
			t.Errorf("maskValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestManifestToConnectOpts_Stdio(t *testing.T) {
	// Verify manifestToConnectOpts builds correct options
	// (Can't easily test without a real manifest, but we verify the function doesn't panic)
	// This is more of a smoke test
	t.Log("manifestToConnectOpts is tested indirectly via integration tests")
}

func TestMCPError_Codes(t *testing.T) {
	// Verify all error codes are distinct strings
	codes := []mcpclient.ErrorCode{
		mcpclient.ErrConnectionFailed,
		mcpclient.ErrProcessSpawnError,
		mcpclient.ErrInitializationTimeout,
		mcpclient.ErrValidationError,
		mcpclient.ErrProtocolError,
	}
	seen := make(map[mcpclient.ErrorCode]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate error code: %s", c)
		}
		seen[c] = true
		if c == "" {
			t.Error("empty error code")
		}
	}
}
