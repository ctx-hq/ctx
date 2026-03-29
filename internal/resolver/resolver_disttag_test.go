package resolver

import (
	"testing"
)

func TestParseRef_WithDistTagSuffix(t *testing.T) {
	tests := []struct {
		input          string
		wantName       string
		wantConstraint string
	}{
		// Dist-tag style references (e.g., @beta)
		{"@scope/name@beta", "@scope/name", "beta"},
		{"@scope/name@latest", "@scope/name", "latest"},
		{"@scope/name@canary", "@scope/name", "canary"},
		{"@scope/name@stable", "@scope/name", "stable"},
		{"@scope/name@rc-1", "@scope/name", "rc-1"},

		// Semver references (should also work)
		{"@scope/name@1.0.0", "@scope/name", "1.0.0"},
		{"@scope/name@^2.0.0", "@scope/name", "^2.0.0"},

		// No constraint
		{"@scope/name", "@scope/name", "*"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, constraint := parseRef(tt.input)
			if name != tt.wantName {
				t.Errorf("parseRef(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
			if constraint != tt.wantConstraint {
				t.Errorf("parseRef(%q) constraint = %q, want %q", tt.input, constraint, tt.wantConstraint)
			}
		})
	}
}

func TestParseRef_GitHubSources(t *testing.T) {
	tests := []struct {
		input          string
		wantName       string
		wantConstraint string
	}{
		{"github:user/repo", "github:user/repo", "*"},
		{"github:org/tool@main", "github:org/tool", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, constraint := parseRef(tt.input)
			if name != tt.wantName {
				t.Errorf("parseRef(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
			if constraint != tt.wantConstraint {
				t.Errorf("parseRef(%q) constraint = %q, want %q", tt.input, constraint, tt.wantConstraint)
			}
		})
	}
}

func TestParseRef_UnscopedPackages(t *testing.T) {
	tests := []struct {
		input          string
		wantName       string
		wantConstraint string
	}{
		{"my-tool", "my-tool", "*"},
		{"my-tool@1.0.0", "my-tool", "1.0.0"},
		{"my-tool@beta", "my-tool", "beta"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, constraint := parseRef(tt.input)
			if name != tt.wantName {
				t.Errorf("parseRef(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
			if constraint != tt.wantConstraint {
				t.Errorf("parseRef(%q) constraint = %q, want %q", tt.input, constraint, tt.wantConstraint)
			}
		})
	}
}
