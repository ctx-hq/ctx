package manifest

import (
	"strings"
	"testing"
)

func TestParseFullName(t *testing.T) {
	tests := []struct {
		input     string
		wantScope string
		wantName  string
	}{
		{"@hong/my-skill", "hong", "my-skill"},
		{"@openelf/code-review", "openelf", "code-review"},
		{"@community/ripgrep", "community", "ripgrep"},
		{"bare-name", "", "bare-name"},
		{"", "", ""},
	}
	for _, tt := range tests {
		scope, name := ParseFullName(tt.input)
		if scope != tt.wantScope || name != tt.wantName {
			t.Errorf("ParseFullName(%q) = (%q, %q), want (%q, %q)", tt.input, scope, name, tt.wantScope, tt.wantName)
		}
	}
}

func TestFormatFullName(t *testing.T) {
	tests := []struct {
		scope, name string
		want        string
	}{
		{"hong", "my-skill", "@hong/my-skill"},
		{"", "bare", "bare"},
	}
	for _, tt := range tests {
		got := FormatFullName(tt.scope, tt.name)
		if got != tt.want {
			t.Errorf("FormatFullName(%q, %q) = %q, want %q", tt.scope, tt.name, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	input := `
name: "@hong/my-skill"
version: "1.0.0"
type: skill
description: "A test skill"
skill:
  entry: SKILL.md
  tags: [test, demo]
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Name != "@hong/my-skill" {
		t.Errorf("Name = %q, want %q", m.Name, "@hong/my-skill")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Type != TypeSkill {
		t.Errorf("Type = %q, want %q", m.Type, TypeSkill)
	}
	if m.Skill == nil {
		t.Fatal("Skill spec is nil")
	}
	if m.Skill.Entry != "SKILL.md" {
		t.Errorf("Skill.Entry = %q, want %q", m.Skill.Entry, "SKILL.md")
	}
	if len(m.Skill.Tags) != 2 {
		t.Errorf("Skill.Tags length = %d, want 2", len(m.Skill.Tags))
	}
}

func TestParseMCP(t *testing.T) {
	input := `
name: "@mcp/github"
version: "2.0.0"
type: mcp
description: "GitHub MCP server"
mcp:
  transport: stdio
  command: npx
  args: ["-y", "@modelcontextprotocol/server-github"]
  env:
    - name: GITHUB_TOKEN
      required: true
      description: "GitHub personal access token"
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Type != TypeMCP {
		t.Errorf("Type = %q, want %q", m.Type, TypeMCP)
	}
	if m.MCP.Transport != "stdio" {
		t.Errorf("MCP.Transport = %q, want %q", m.MCP.Transport, "stdio")
	}
	if m.MCP.Command != "npx" {
		t.Errorf("MCP.Command = %q, want %q", m.MCP.Command, "npx")
	}
	if len(m.MCP.Env) != 1 || m.MCP.Env[0].Name != "GITHUB_TOKEN" {
		t.Errorf("MCP.Env unexpected: %+v", m.MCP.Env)
	}
}

func TestParseCLI(t *testing.T) {
	input := `
name: "@community/ripgrep"
version: "14.1.0"
type: cli
description: "Fast regex search tool"
cli:
  binary: rg
  verify: "rg --version"
install:
  brew: "ripgrep"
  cargo: "ripgrep"
  platforms:
    darwin-arm64:
      binary: "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-aarch64-apple-darwin.tar.gz"
    linux-amd64:
      binary: "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz"
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.CLI.Binary != "rg" {
		t.Errorf("CLI.Binary = %q, want %q", m.CLI.Binary, "rg")
	}
	if m.Install == nil {
		t.Fatal("Install is nil")
	}
	if m.Install.Brew != "ripgrep" {
		t.Errorf("Install.Brew = %q, want %q", m.Install.Brew, "ripgrep")
	}
	if len(m.Install.Platforms) != 2 {
		t.Errorf("Install.Platforms length = %d, want 2", len(m.Install.Platforms))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr int // expected number of errors
	}{
		{
			name:    "empty manifest",
			m:       Manifest{},
			wantErr: 4, // name, version, type, description
		},
		{
			name: "valid skill",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeSkill,
				Description: "A test skill",
			},
			wantErr: 0,
		},
		{
			name: "invalid name format",
			m: Manifest{
				Name:        "BadName",
				Version:     "1.0.0",
				Type:        TypeSkill,
				Description: "test",
			},
			wantErr: 1,
		},
		{
			name: "invalid version",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "v1.0",
				Type:        TypeSkill,
				Description: "test",
			},
			wantErr: 1,
		},
		{
			name: "mcp missing section",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeMCP,
				Description: "test",
			},
			wantErr: 1,
		},
		{
			name: "mcp missing transport",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeMCP,
				Description: "test",
				MCP:         &MCPSpec{},
			},
			wantErr: 1,
		},
		{
			name: "mcp stdio missing command",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeMCP,
				Description: "test",
				MCP:         &MCPSpec{Transport: "stdio"},
			},
			wantErr: 1,
		},
		{
			name: "valid mcp stdio",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeMCP,
				Description: "test",
				MCP:         &MCPSpec{Transport: "stdio", Command: "node"},
			},
			wantErr: 0,
		},
		{
			name: "cli missing section",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeCLI,
				Description: "test",
			},
			wantErr: 1,
		},
		{
			name: "cli missing binary",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeCLI,
				Description: "test",
				CLI:         &CLISpec{},
			},
			wantErr: 1,
		},
		{
			name: "valid cli",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeCLI,
				Description: "test",
				CLI:         &CLISpec{Binary: "rg"},
			},
			wantErr: 0,
		},
		{
			name: "prerelease version",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0-beta.1",
				Type:        TypeSkill,
				Description: "test",
			},
			wantErr: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.m)
			if len(errs) != tt.wantErr {
				t.Errorf("Validate() returned %d errors, want %d: %v", len(errs), tt.wantErr, errs)
			}
		})
	}
}

func TestScaffold(t *testing.T) {
	m := Scaffold(TypeSkill, "hong", "my-skill")
	if m.Name != "@hong/my-skill" {
		t.Errorf("Name = %q, want %q", m.Name, "@hong/my-skill")
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", m.Version, "0.1.0")
	}
	if m.Skill == nil {
		t.Error("Skill should not be nil")
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Scaffolded manifest has errors: %v", errs)
	}
}

func TestMarshalRoundtrip(t *testing.T) {
	m := Scaffold(TypeMCP, "test", "server")
	m.MCP = &MCPSpec{Transport: "stdio", Command: "node"}
	m.Description = "test server"

	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	m2, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m2.Name != m.Name {
		t.Errorf("roundtrip Name = %q, want %q", m2.Name, m.Name)
	}
	if m2.Type != m.Type {
		t.Errorf("roundtrip Type = %q, want %q", m2.Type, m.Type)
	}
}
