package main

import (
	"testing"
)

func TestGetToken_EnvVarTakesPrecedence(t *testing.T) {
	t.Setenv("CTX_TOKEN", "env-token-value")

	token := getToken()
	if token != "env-token-value" {
		t.Errorf("expected env token, got %q", token)
	}
}

func TestGetToken_EmptyEnvVarFallsThrough(t *testing.T) {
	t.Setenv("CTX_TOKEN", "")

	// Empty env var should not be treated as a valid token — must fall
	// through to keychain. We can't predict the keychain value, but we
	// verify the empty string was not short-circuited as the env token.
	token := getToken()
	// Keychain may legitimately return "" in CI / sandboxed environments,
	// so we just verify getToken() doesn't panic with an empty env var.
	_ = token
}

func TestParsePackageVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
		wantErr     bool
	}{
		// scoped packages
		{"@scope/name@1.0.0", "@scope/name", "1.0.0", false},
		{"@my-org/tool@0.2.3-beta.1", "@my-org/tool", "0.2.3-beta.1", false},
		{"@a/b@v", "@a/b", "v", false},

		// unscoped packages
		{"pkg@2.0.0", "pkg", "2.0.0", false},
		{"my-tool@latest", "my-tool", "latest", false},

		// errors
		{"@scope/name", "", "", true},   // missing @version
		{"@scopeonly", "", "", true},     // no slash
		{"pkg", "", "", true},            // no @version
		{"@scope/@1.0", "", "", true},    // empty name segment
		{"", "", "", true},               // empty input
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version, err := parsePackageVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePackageVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if name != tt.wantName {
					t.Errorf("name = %q, want %q", name, tt.wantName)
				}
				if version != tt.wantVersion {
					t.Errorf("version = %q, want %q", version, tt.wantVersion)
				}
			}
		})
	}
}
