package main

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestInitModeDetection_ModeA_FromScratch(t *testing.T) {
	// Mode A: no ctx.yaml, no SKILL.md => scaffold from scratch
	// We test the scaffold output for each valid type
	types := []struct {
		pkgType manifest.PackageType
		wantNil map[string]bool // which spec should be nil
	}{
		{manifest.TypeSkill, map[string]bool{"MCP": true, "CLI": true}},
		{manifest.TypeMCP, map[string]bool{"Skill": true, "CLI": true}},
		{manifest.TypeCLI, map[string]bool{"Skill": true, "MCP": true}},
	}

	for _, tt := range types {
		t.Run(string(tt.pkgType), func(t *testing.T) {
			m := manifest.Scaffold(tt.pkgType, "testuser", "my-pkg")

			if m.Name != "@testuser/my-pkg" {
				t.Errorf("Name = %q, want %q", m.Name, "@testuser/my-pkg")
			}
			if m.Version != "0.1.0" {
				t.Errorf("Version = %q, want %q", m.Version, "0.1.0")
			}
			if m.Type != tt.pkgType {
				t.Errorf("Type = %q, want %q", m.Type, tt.pkgType)
			}

			// Verify only the correct spec is non-nil
			if (m.Skill == nil) != tt.wantNil["Skill"] {
				t.Errorf("Skill nil = %v, want %v", m.Skill == nil, tt.wantNil["Skill"])
			}
			if (m.MCP == nil) != tt.wantNil["MCP"] {
				t.Errorf("MCP nil = %v, want %v", m.MCP == nil, tt.wantNil["MCP"])
			}
			if (m.CLI == nil) != tt.wantNil["CLI"] {
				t.Errorf("CLI nil = %v, want %v", m.CLI == nil, tt.wantNil["CLI"])
			}
		})
	}
}

func TestInitModeDetection_PackageTypeValid(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"skill", true},
		{"mcp", true},
		{"cli", true},
		{"tool", false},
		{"plugin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pt := manifest.PackageType(tt.input)
			if pt.Valid() != tt.valid {
				t.Errorf("PackageType(%q).Valid() = %v, want %v", tt.input, pt.Valid(), tt.valid)
			}
		})
	}
}

func TestInitCmd_DefaultType(t *testing.T) {
	f := initCmd.Flags().Lookup("type")
	if f == nil {
		t.Fatal("--type flag not found on init command")
	}
	if f.DefValue != "skill" {
		t.Errorf("default type = %q, want %q", f.DefValue, "skill")
	}
}
