package resolver

import (
	"strings"
	"testing"
)

func TestParseRef_GitHub(t *testing.T) {
	tests := []struct {
		input      string
		wantName   string
		wantConstr string
	}{
		{"github:user/repo", "github:user/repo", "*"},
		{"github:user/repo@v1.0.0", "github:user/repo", "v1.0.0"},
		{"github:user/repo@main", "github:user/repo", "main"},
		{"@scope/name", "@scope/name", "*"},
		{"@scope/name@^1.0", "@scope/name", "^1.0"},
		{"@scope/name@1.2.3", "@scope/name", "1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, constr := parseRef(tt.input)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if constr != tt.wantConstr {
				t.Errorf("constraint = %q, want %q", constr, tt.wantConstr)
			}
		})
	}
}

func TestParseRef_GitHubURLConstruction(t *testing.T) {
	// Test that github: prefix is correctly identified
	name, _ := parseRef("github:basecamp/basecamp-cli")
	if !strings.HasPrefix(name, "github:") {
		t.Errorf("expected github: prefix, got %q", name)
	}

	// Test with ref
	name2, ref := parseRef("github:basecamp/basecamp-cli@v0.7.2")
	if !strings.HasPrefix(name2, "github:") {
		t.Errorf("expected github: prefix, got %q", name2)
	}
	if ref != "v0.7.2" {
		t.Errorf("ref = %q, want v0.7.2", ref)
	}
}

func TestParseRef_ScopedPackage(t *testing.T) {
	// Scoped packages have @ at start, version constraint after second @
	name, constr := parseRef("@ctx/basecamp@^1.0.0")
	if name != "@ctx/basecamp" {
		t.Errorf("name = %q, want @ctx/basecamp", name)
	}
	if constr != "^1.0.0" {
		t.Errorf("constraint = %q, want ^1.0.0", constr)
	}
}

func TestParseRef_NoConstraint(t *testing.T) {
	name, constr := parseRef("@ctx/basecamp")
	if name != "@ctx/basecamp" {
		t.Errorf("name = %q", name)
	}
	if constr != "*" {
		t.Errorf("constraint should be '*' (default), got %q", constr)
	}
}
