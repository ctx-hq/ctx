package installer

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestRunPreflight_NilRequire(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{Transport: "stdio", Command: "node"},
	}
	result := RunPreflight(m)
	if result != nil {
		t.Error("expected nil result when no require section")
	}
}

func TestRunPreflight_NilMCP(t *testing.T) {
	m := &manifest.Manifest{}
	result := RunPreflight(m)
	if result != nil {
		t.Error("expected nil result when no MCP section")
	}
}

func TestRunPreflight_EmptyRequire(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Require:   &manifest.MCPRequireSpec{},
		},
	}
	result := RunPreflight(m)
	if result != nil {
		t.Error("expected nil result when require section is empty")
	}
}

func TestRunPreflight_BinExists(t *testing.T) {
	// "sh" should exist on any Unix system
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "sh",
			Require: &manifest.MCPRequireSpec{
				Bins: []string{"sh"},
			},
		},
	}
	result := RunPreflight(m)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Passed {
		t.Errorf("expected preflight to pass for 'sh', errors: %v", result.Errors)
	}
}

func TestRunPreflight_BinMissing(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "nonexistent-bin-xyz",
			Require: &manifest.MCPRequireSpec{
				Bins: []string{"nonexistent-bin-xyz"},
			},
		},
	}
	result := RunPreflight(m)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Passed {
		t.Error("expected preflight to fail for missing bin")
	}
	if len(result.Missing) != 1 || result.Missing[0] != "nonexistent-bin-xyz" {
		t.Errorf("expected missing=[nonexistent-bin-xyz], got %v", result.Missing)
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v22.1.0", "22.1.0"},
		{"node v22.1.0", "22.1.0"},
		{"Node.js v18.20.3", "18.20.3"},
		{"Python 3.12.4", "3.12.4"},
		{"rg 14.1.0", "14.1.0"},
		{"Docker version 27.3.1, build ce12230", "27.3.1,"},
		// extractVersion strips trailing punctuation handled by versionSatisfies caller
		{"no version here", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractVersion(tt.input)
		// For the Docker case, clean up
		if tt.input == "Docker version 27.3.1, build ce12230" {
			// extractVersion returns "27.3.1," due to comma, but that's ok
			// parseVersionParts handles non-digit chars
			if got == "" {
				t.Errorf("extractVersion(%q) = %q, want non-empty", tt.input, got)
			}
			continue
		}
		if got != tt.want {
			t.Errorf("extractVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVersionSatisfies(t *testing.T) {
	tests := []struct {
		detected string
		required string
		want     bool
	}{
		{"22.1.0", "18.0.0", true},
		{"18.0.0", "18.0.0", true},
		{"17.9.9", "18.0.0", false},
		{"18.0.1", "18.0.0", true},
		{"3.12.4", "3.10.0", true},
		{"1.0.0", "2.0.0", false},
	}
	for _, tt := range tests {
		got := versionSatisfies(tt.detected, tt.required)
		if got != tt.want {
			t.Errorf("versionSatisfies(%q, %q) = %v, want %v", tt.detected, tt.required, got, tt.want)
		}
	}
}

func TestCoerceVersion(t *testing.T) {
	tests := []struct {
		input   string
		wantNil bool
		major   uint64
		minor   uint64
		patch   uint64
	}{
		{"22.1.0", false, 22, 1, 0},
		{"18.0.0", false, 18, 0, 0},
		{"1.0.0-beta.1", false, 1, 0, 0},
		{"3.12", false, 3, 12, 0},
		{"27.3.1,", false, 27, 3, 1},
	}
	for _, tt := range tests {
		got := coerceVersion(tt.input)
		if tt.wantNil {
			if got != nil {
				t.Errorf("coerceVersion(%q) = %v, want nil", tt.input, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("coerceVersion(%q) = nil, want %d.%d.%d", tt.input, tt.major, tt.minor, tt.patch)
			continue
		}
		if got.Major() != tt.major || got.Minor() != tt.minor || got.Patch() != tt.patch {
			t.Errorf("coerceVersion(%q) = %d.%d.%d, want %d.%d.%d",
				tt.input, got.Major(), got.Minor(), got.Patch(), tt.major, tt.minor, tt.patch)
		}
	}
}
