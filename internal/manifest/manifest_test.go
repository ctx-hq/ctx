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
skill:
  entry: "skills/github/SKILL.md"
  origin: native
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
skill:
  entry: "skills/ripgrep/SKILL.md"
  origin: native
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
			wantErr: 5, // name, version, type, description, skill
		},
		{
			name: "valid skill",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeSkill,
				Description: "A test skill",
				Skill:       &SkillSpec{Entry: "SKILL.md"},
			},
			wantErr: 0,
		},
		{
			name: "missing skill section",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeSkill,
				Description: "test",
			},
			wantErr: 1, // skill section required
		},
		{
			name: "invalid name format",
			m: Manifest{
				Name:        "BadName",
				Version:     "1.0.0",
				Type:        TypeSkill,
				Description: "test",
				Skill:       &SkillSpec{Entry: "SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "SKILL.md"},
			},
			wantErr: 1,
		},
		{
			name: "mcp missing mcp section",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeMCP,
				Description: "test",
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
				MCP:         &MCPSpec{Transport: "stdio", Command: "node"},
			},
			wantErr: 0,
		},
		{
			name: "cli missing cli section",
			m: Manifest{
				Name:        "@hong/test",
				Version:     "1.0.0",
				Type:        TypeCLI,
				Description: "test",
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
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
				Skill:       &SkillSpec{Entry: "SKILL.md"},
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

func TestParseCLIWithSkill(t *testing.T) {
	input := `
name: "@biao29/fizzy-cli"
version: "0.1.0"
type: cli
description: "CLI for Fizzy project management"
cli:
  binary: fizzy
  verify: "fizzy --version"
skill:
  entry: "skills/fizzy/SKILL.md"
  origin: native
  tags: [project-management, kanban]
  user_invocable: true
install:
  brew: "basecamp/tap/fizzy"
  script: "https://example.com/install.sh"
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Type != TypeCLI {
		t.Errorf("Type = %q, want %q", m.Type, TypeCLI)
	}
	if m.CLI == nil || m.CLI.Binary != "fizzy" {
		t.Errorf("CLI.Binary = %v, want fizzy", m.CLI)
	}
	if m.Skill == nil {
		t.Fatal("Skill should not be nil for CLI+Skill package")
	}
	if m.Skill.Origin != "native" {
		t.Errorf("Skill.Origin = %q, want %q", m.Skill.Origin, "native")
	}
	if m.Skill.Entry != "skills/fizzy/SKILL.md" {
		t.Errorf("Skill.Entry = %q, want %q", m.Skill.Entry, "skills/fizzy/SKILL.md")
	}
	if len(m.Skill.Tags) != 2 {
		t.Errorf("Skill.Tags length = %d, want 2", len(m.Skill.Tags))
	}
	if m.Install.Script != "https://example.com/install.sh" {
		t.Errorf("Install.Script = %q, want %q", m.Install.Script, "https://example.com/install.sh")
	}

	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Valid CLI+Skill manifest has errors: %v", errs)
	}
}

func TestValidateCLISkillOriginAndScript(t *testing.T) {
	base := func() Manifest {
		return Manifest{
			Name:        "@hong/test",
			Version:     "1.0.0",
			Type:        TypeCLI,
			Description: "test",
			CLI:         &CLISpec{Binary: "test"},
			Skill:       &SkillSpec{Entry: "skills/test/SKILL.md"},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Manifest)
		wantErr int
	}{
		{
			name: "cli with skill section valid",
			modify: func(m *Manifest) {
				m.Skill = &SkillSpec{Entry: "SKILL.md", Origin: "native"}
			},
			wantErr: 0,
		},
		{
			name: "cli with skill origin wrapped",
			modify: func(m *Manifest) {
				m.Skill = &SkillSpec{Entry: "SKILL.md", Origin: "wrapped"}
			},
			wantErr: 0,
		},
		{
			name: "cli with skill origin empty",
			modify: func(m *Manifest) {
				m.Skill = &SkillSpec{Entry: "SKILL.md"}
			},
			wantErr: 0,
		},
		{
			name: "cli with invalid skill origin",
			modify: func(m *Manifest) {
				m.Skill = &SkillSpec{Entry: "SKILL.md", Origin: "invalid"}
			},
			wantErr: 1,
		},
		{
			name: "install script http rejected",
			modify: func(m *Manifest) {
				m.Install = &InstallSpec{Script: "http://example.com/install.sh"}
			},
			wantErr: 1,
		},
		{
			name: "install script https accepted",
			modify: func(m *Manifest) {
				m.Install = &InstallSpec{Script: "https://example.com/install.sh"}
			},
			wantErr: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.modify(&m)
			errs := Validate(&m)
			if len(errs) != tt.wantErr {
				t.Errorf("Validate() returned %d errors, want %d: %v", len(errs), tt.wantErr, errs)
			}
		})
	}
}

func TestMarshalRoundtripCLIWithSkill(t *testing.T) {
	m := &Manifest{
		Name:        "@hong/fizzy",
		Version:     "1.0.0",
		Type:        TypeCLI,
		Description: "test cli with skill",
		CLI:         &CLISpec{Binary: "fizzy", Verify: "fizzy --version"},
		Skill:       &SkillSpec{Entry: "SKILL.md", Origin: "native", Tags: []string{"cli", "agent"}},
		Install:     &InstallSpec{Brew: "fizzy", Script: "https://example.com/install.sh"},
	}

	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	m2, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m2.CLI == nil || m2.CLI.Binary != "fizzy" {
		t.Errorf("roundtrip CLI.Binary = %v", m2.CLI)
	}
	if m2.Skill == nil || m2.Skill.Origin != "native" {
		t.Errorf("roundtrip Skill.Origin = %v", m2.Skill)
	}
	if m2.Install == nil || m2.Install.Script != "https://example.com/install.sh" {
		t.Errorf("roundtrip Install.Script = %v", m2.Install)
	}
}

func TestPackageFilesIncludesLicense(t *testing.T) {
	tests := []struct {
		name string
		m    Manifest
	}{
		{
			name: "skill",
			m:    Manifest{Type: TypeSkill, Skill: &SkillSpec{Entry: "SKILL.md"}},
		},
		{
			name: "cli",
			m:    Manifest{Type: TypeCLI, Skill: &SkillSpec{Entry: "skills/test/SKILL.md"}, CLI: &CLISpec{Binary: "test"}},
		},
		{
			name: "mcp",
			m:    Manifest{Type: TypeMCP, MCP: &MCPSpec{Transport: "stdio", Command: "node"}},
		},
	}

	licenseCandidates := map[string]bool{
		"LICENSE": true, "LICENSE.md": true, "LICENSE.txt": true,
		"LICENCE": true, "LICENCE.md": true, "LICENCE.txt": true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.m.PackageFiles()
			found := 0
			for _, f := range files {
				if licenseCandidates[f] {
					found++
				}
			}
			if found != len(licenseCandidates) {
				t.Errorf("PackageFiles() contains %d license candidates, want %d", found, len(licenseCandidates))
			}
		})
	}
}

func TestParseWorkspace(t *testing.T) {
	input := `
name: "@baoyu/skills"
type: workspace
description: "Baoyu's skill collection"
workspace:
  members:
    - "skills/*"
  exclude:
    - "docs"
  defaults:
    scope: "@baoyu"
    author: "Jim Liu"
    license: MIT
  collections:
    - name: document-skills
      description: "Document processing"
      members: [xlsx, docx, pdf]
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Type != TypeWorkspace {
		t.Errorf("Type = %q, want workspace", m.Type)
	}
	if m.Workspace == nil {
		t.Fatal("Workspace is nil")
	}
	if len(m.Workspace.Members) != 1 || m.Workspace.Members[0] != "skills/*" {
		t.Errorf("Workspace.Members = %v, want [skills/*]", m.Workspace.Members)
	}
	if len(m.Workspace.Exclude) != 1 || m.Workspace.Exclude[0] != "docs" {
		t.Errorf("Workspace.Exclude = %v, want [docs]", m.Workspace.Exclude)
	}
	if m.Workspace.Defaults == nil || m.Workspace.Defaults.Scope != "@baoyu" {
		t.Errorf("Workspace.Defaults.Scope = %v", m.Workspace.Defaults)
	}
	if len(m.Workspace.Collections) != 1 {
		t.Fatalf("Collections count = %d, want 1", len(m.Workspace.Collections))
	}
	c := m.Workspace.Collections[0]
	if c.Name != "document-skills" {
		t.Errorf("Collection.Name = %q", c.Name)
	}
	if len(c.Members) != 3 {
		t.Errorf("Collection.Members = %v, want 3 items", c.Members)
	}
}

func TestParseCollection(t *testing.T) {
	input := `
name: "@baoyu/skills"
version: "1.0.0"
type: collection
description: "All Baoyu skills"
collection:
  members:
    - "@baoyu/translate"
    - "@baoyu/comic"
    - "@baoyu/infographic"
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Type != TypeCollection {
		t.Errorf("Type = %q, want collection", m.Type)
	}
	if m.Collection == nil {
		t.Fatal("Collection is nil")
	}
	if len(m.Collection.Members) != 3 {
		t.Errorf("Collection.Members count = %d, want 3", len(m.Collection.Members))
	}
}

func TestValidateWorkspace_Valid(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
		Workspace: &WorkspaceSpec{
			Members: []string{"skills/*"},
		},
	}
	errs := Validate(&m)
	if len(errs) != 0 {
		t.Errorf("Validate() returned errors: %v", errs)
	}
}

func TestValidateWorkspace_MissingMembers(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
		Workspace:   &WorkspaceSpec{},
	}
	errs := Validate(&m)
	if len(errs) != 1 {
		t.Errorf("Validate() returned %d errors, want 1: %v", len(errs), errs)
	}
}

func TestValidateWorkspace_MissingSection(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
	}
	errs := Validate(&m)
	if len(errs) != 1 {
		t.Errorf("Validate() returned %d errors, want 1: %v", len(errs), errs)
	}
}

func TestValidateWorkspace_NoVersionRequired(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
		Workspace: &WorkspaceSpec{
			Members: []string{"skills/*"},
		},
	}
	errs := Validate(&m)
	if len(errs) != 0 {
		t.Errorf("Workspace should not require version, got errors: %v", errs)
	}
}

func TestValidateWorkspace_RejectsSkillSection(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
		Workspace:   &WorkspaceSpec{Members: []string{"*"}},
		Skill:       &SkillSpec{Entry: "SKILL.md"},
	}
	errs := Validate(&m)
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e, "cannot have skill") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected error about skill section on workspace, got: %v", errs)
	}
}

func TestValidateCollection_Valid(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Version:     "1.0.0",
		Type:        TypeCollection,
		Description: "Test collection",
		Collection:  &CollectionManifest{Members: []string{"@test/alpha", "@test/beta"}},
	}
	errs := Validate(&m)
	if len(errs) != 0 {
		t.Errorf("Validate() returned errors: %v", errs)
	}
}

func TestValidateCollection_InvalidMemberName(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Version:     "1.0.0",
		Type:        TypeCollection,
		Description: "Test collection",
		Collection:  &CollectionManifest{Members: []string{"BadName"}},
	}
	errs := Validate(&m)
	if len(errs) != 1 {
		t.Errorf("Validate() returned %d errors, want 1: %v", len(errs), errs)
	}
}

func TestValidateCollection_EmptyMembers(t *testing.T) {
	m := Manifest{
		Name:        "@test/skills",
		Version:     "1.0.0",
		Type:        TypeCollection,
		Description: "Test collection",
	}
	errs := Validate(&m)
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e, "collection.members") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected collection.members error, got: %v", errs)
	}
}

func TestMarshalRoundtripWorkspace(t *testing.T) {
	m := &Manifest{
		Name:        "@test/skills",
		Type:        TypeWorkspace,
		Description: "Test workspace",
		Workspace: &WorkspaceSpec{
			Members: []string{"skills/*"},
			Exclude: []string{"docs"},
			Defaults: &WorkspaceDefaults{
				Scope:  "@test",
				Author: "Test Author",
			},
			Collections: []CollectionSpec{
				{Name: "docs", Description: "Document skills", Members: []string{"xlsx", "pdf"}},
			},
		},
	}

	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	m2, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m2.Type != TypeWorkspace {
		t.Errorf("roundtrip Type = %q, want workspace", m2.Type)
	}
	if m2.Workspace == nil {
		t.Fatal("roundtrip Workspace is nil")
	}
	if len(m2.Workspace.Members) != 1 {
		t.Errorf("roundtrip Members = %v", m2.Workspace.Members)
	}
	if len(m2.Workspace.Collections) != 1 {
		t.Errorf("roundtrip Collections = %v", m2.Workspace.Collections)
	}
}

func TestParseMCPReference(t *testing.T) {
	input := `
name: "@mcp/playwright"
version: "0.0.70"
type: mcp
description: "Browser automation MCP server using Playwright"

upstream:
  npm: "@playwright/mcp"
  tracking: npm
  version_pattern: "*"

mcp:
  transport: stdio
  command: npx
  args: ["-y", "@playwright/mcp@0.0.70"]
  env:
    - name: PLAYWRIGHT_MCP_HEADLESS
      required: false
      default: "true"
      description: "Run browser in headless mode"
  tools:
    - browser_navigate
    - browser_click
  require:
    bins: [node]
    min_versions:
      node: "18.0.0"
  hooks:
    post_install:
      - command: npx
        args: ["playwright", "install", "chromium"]
        description: "Install Chromium browser binary"
        optional: false
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	// Upstream
	if m.Upstream == nil {
		t.Fatal("Upstream is nil")
	}
	if m.Upstream.NPM != "@playwright/mcp" {
		t.Errorf("Upstream.NPM = %q, want %q", m.Upstream.NPM, "@playwright/mcp")
	}
	if m.Upstream.Tracking != "npm" {
		t.Errorf("Upstream.Tracking = %q, want %q", m.Upstream.Tracking, "npm")
	}
	// Require
	if m.MCP.Require == nil {
		t.Fatal("MCP.Require is nil")
	}
	if len(m.MCP.Require.Bins) != 1 || m.MCP.Require.Bins[0] != "node" {
		t.Errorf("MCP.Require.Bins = %v, want [node]", m.MCP.Require.Bins)
	}
	if m.MCP.Require.MinVersions["node"] != "18.0.0" {
		t.Errorf("MCP.Require.MinVersions[node] = %q, want %q", m.MCP.Require.MinVersions["node"], "18.0.0")
	}
	// Hooks
	if m.MCP.Hooks == nil {
		t.Fatal("MCP.Hooks is nil")
	}
	if len(m.MCP.Hooks.PostInstall) != 1 {
		t.Fatalf("PostInstall count = %d, want 1", len(m.MCP.Hooks.PostInstall))
	}
	h := m.MCP.Hooks.PostInstall[0]
	if h.Command != "npx" {
		t.Errorf("Hook.Command = %q, want %q", h.Command, "npx")
	}
	if len(h.Args) != 3 {
		t.Errorf("Hook.Args = %v, want 3 args", h.Args)
	}
	if h.Optional {
		t.Error("Hook.Optional should be false")
	}
	// Tools
	if len(m.MCP.Tools) != 2 {
		t.Errorf("MCP.Tools = %v, want 2", m.MCP.Tools)
	}
	// Validation
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Validate() errors: %v", errs)
	}
}

func TestParseMCPMultiTransport(t *testing.T) {
	input := `
name: "@mcp/github"
version: "0.2.0"
type: mcp
description: "Connect AI assistants to GitHub"

upstream:
  github: "github/github-mcp-server"
  docker: "ghcr.io/github/github-mcp-server"
  tracking: github-release

mcp:
  transport: stdio
  command: docker
  args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server:v0.2.0"]
  env:
    - name: GITHUB_PERSONAL_ACCESS_TOKEN
      required: true
      description: "GitHub personal access token"
  require:
    bins: [docker]
  transports:
    - id: stdio-docker
      label: "Docker (stdio)"
      transport: stdio
      command: docker
      args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server:v0.2.0"]
      require:
        bins: [docker]
    - id: remote
      label: "GitHub Copilot (remote)"
      transport: streamable-http
      url: "https://api.githubcopilot.com/mcp/"
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	// Transports
	if len(m.MCP.Transports) != 2 {
		t.Fatalf("Transports count = %d, want 2", len(m.MCP.Transports))
	}
	t0 := m.MCP.Transports[0]
	if t0.ID != "stdio-docker" {
		t.Errorf("Transport[0].ID = %q, want %q", t0.ID, "stdio-docker")
	}
	if t0.Transport != "stdio" {
		t.Errorf("Transport[0].Transport = %q, want %q", t0.Transport, "stdio")
	}
	if t0.Command != "docker" {
		t.Errorf("Transport[0].Command = %q, want %q", t0.Command, "docker")
	}
	if t0.Require == nil || len(t0.Require.Bins) != 1 {
		t.Errorf("Transport[0].Require.Bins = %v", t0.Require)
	}

	t1 := m.MCP.Transports[1]
	if t1.ID != "remote" {
		t.Errorf("Transport[1].ID = %q, want %q", t1.ID, "remote")
	}
	if t1.Transport != "streamable-http" {
		t.Errorf("Transport[1].Transport = %q, want %q", t1.Transport, "streamable-http")
	}
	if t1.URL != "https://api.githubcopilot.com/mcp/" {
		t.Errorf("Transport[1].URL = %q", t1.URL)
	}

	// Upstream
	if m.Upstream == nil || m.Upstream.GitHub != "github/github-mcp-server" {
		t.Errorf("Upstream.GitHub = %v", m.Upstream)
	}
	if m.Upstream.Tracking != "github-release" {
		t.Errorf("Upstream.Tracking = %q", m.Upstream.Tracking)
	}

	// Validation should pass
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Validate() errors: %v", errs)
	}
}

func TestValidateTransportErrors(t *testing.T) {
	base := func() Manifest {
		return Manifest{
			Name:        "@mcp/test",
			Version:     "1.0.0",
			Type:        TypeMCP,
			Description: "test",
			MCP: &MCPSpec{
				Transport: "stdio",
				Command:   "node",
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Manifest)
		wantErr int
	}{
		{
			name: "transport missing id",
			modify: func(m *Manifest) {
				m.MCP.Transports = []TransportSpec{{Transport: "stdio", Command: "node"}}
			},
			wantErr: 1,
		},
		{
			name: "transport duplicate id",
			modify: func(m *Manifest) {
				m.MCP.Transports = []TransportSpec{
					{ID: "a", Transport: "stdio", Command: "node"},
					{ID: "a", Transport: "stdio", Command: "docker"},
				}
			},
			wantErr: 1,
		},
		{
			name: "transport invalid type",
			modify: func(m *Manifest) {
				m.MCP.Transports = []TransportSpec{{ID: "a", Transport: "invalid"}}
			},
			wantErr: 1,
		},
		{
			name: "transport stdio missing command",
			modify: func(m *Manifest) {
				m.MCP.Transports = []TransportSpec{{ID: "a", Transport: "stdio"}}
			},
			wantErr: 1,
		},
		{
			name: "transport http missing url",
			modify: func(m *Manifest) {
				m.MCP.Transports = []TransportSpec{{ID: "a", Transport: "streamable-http"}}
			},
			wantErr: 1,
		},
		{
			name: "hook missing command",
			modify: func(m *Manifest) {
				m.MCP.Hooks = &MCPHooks{PostInstall: []HookStep{{}}}
			},
			wantErr: 1,
		},
		{
			name: "upstream empty sources",
			modify: func(m *Manifest) {
				m.Upstream = &UpstreamSpec{}
			},
			wantErr: 1,
		},
		{
			name: "upstream invalid tracking",
			modify: func(m *Manifest) {
				m.Upstream = &UpstreamSpec{NPM: "@test/pkg", Tracking: "invalid"}
			},
			wantErr: 1,
		},
		{
			name: "upstream valid",
			modify: func(m *Manifest) {
				m.Upstream = &UpstreamSpec{NPM: "@test/pkg", Tracking: "npm"}
			},
			wantErr: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.modify(&m)
			errs := Validate(&m)
			if len(errs) != tt.wantErr {
				t.Errorf("Validate() returned %d errors, want %d: %v", len(errs), tt.wantErr, errs)
			}
		})
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	// Existing MCP manifests without new fields should still parse and validate fine
	input := `
name: "@test/github-mcp"
version: "2.1.0"
type: mcp
description: "GitHub MCP server for repository operations"
keywords: [github, git, mcp]
skill:
  entry: "skills/github-mcp/SKILL.md"
  origin: native
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
	if m.Upstream != nil {
		t.Error("Upstream should be nil for old manifest")
	}
	if m.MCP.Require != nil {
		t.Error("MCP.Require should be nil for old manifest")
	}
	if m.MCP.Hooks != nil {
		t.Error("MCP.Hooks should be nil for old manifest")
	}
	if len(m.MCP.Transports) != 0 {
		t.Error("MCP.Transports should be empty for old manifest")
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Old manifest should validate: %v", errs)
	}
}

func TestMarshalRoundtripMCPReference(t *testing.T) {
	m := &Manifest{
		Name:        "@mcp/playwright",
		Version:     "0.0.70",
		Type:        TypeMCP,
		Description: "Browser automation",
		Upstream:    &UpstreamSpec{NPM: "@playwright/mcp", Tracking: "npm"},
		MCP: &MCPSpec{
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", "@playwright/mcp@0.0.70"},
			Require:   &MCPRequireSpec{Bins: []string{"node"}, MinVersions: map[string]string{"node": "18.0.0"}},
			Hooks: &MCPHooks{
				PostInstall: []HookStep{
					{Command: "npx", Args: []string{"playwright", "install"}, Description: "Install browser"},
				},
			},
			Transports: []TransportSpec{
				{ID: "default", Transport: "stdio", Command: "npx"},
			},
		},
	}

	data, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	m2, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m2.Upstream == nil || m2.Upstream.NPM != "@playwright/mcp" {
		t.Errorf("roundtrip Upstream.NPM = %v", m2.Upstream)
	}
	if m2.MCP.Require == nil || len(m2.MCP.Require.Bins) != 1 {
		t.Errorf("roundtrip MCP.Require.Bins = %v", m2.MCP.Require)
	}
	if m2.MCP.Hooks == nil || len(m2.MCP.Hooks.PostInstall) != 1 {
		t.Errorf("roundtrip MCP.Hooks = %v", m2.MCP.Hooks)
	}
	if len(m2.MCP.Transports) != 1 || m2.MCP.Transports[0].ID != "default" {
		t.Errorf("roundtrip MCP.Transports = %v", m2.MCP.Transports)
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

func TestValidateUpstreamVersionPattern(t *testing.T) {
	base := func() Manifest {
		return Manifest{
			Name:        "@mcp/test",
			Version:     "1.0.0",
			Type:        TypeMCP,
			Description: "test",
			MCP: &MCPSpec{
				Transport: "stdio",
				Command:   "node",
			},
		}
	}

	tests := []struct {
		name    string
		pattern string
		wantErr int
	}{
		{"wildcard *", "*", 0},
		{"semver constraint ^1.0.0", "^1.0.0", 0},
		{"semver constraint >=2.0.0", ">=2.0.0", 0},
		{"semver constraint ~1.2.3", "~1.2.3", 0},
		{"exact semver 1.2.3", "1.2.3", 0},
		{"invalid pattern", "latest", 1},
		{"invalid nonsense", "abc.xyz", 1},
		{"empty (valid — optional field)", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			m.Upstream = &UpstreamSpec{NPM: "@test/pkg", Tracking: "npm", VersionPattern: tt.pattern}
			errs := Validate(&m)
			if len(errs) != tt.wantErr {
				t.Errorf("Validate() with version_pattern=%q returned %d errors, want %d: %v", tt.pattern, len(errs), tt.wantErr, errs)
			}
		})
	}
}

func TestValidateCommandSafety(t *testing.T) {
	base := func() Manifest {
		return Manifest{
			Name:        "@mcp/test",
			Version:     "1.0.0",
			Type:        TypeMCP,
			Description: "test",
			MCP: &MCPSpec{
				Transport: "stdio",
				Command:   "node",
			},
		}
	}

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"normal command", "npx", false},
		{"path command", "node", false},
		{"pipe injection", "node | cat /etc/passwd", true},
		{"semicolon injection", "node; rm -rf /", true},
		{"backtick injection", "node `whoami`", true},
		{"dollar injection", "node $HOME", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			m.MCP.Command = tt.command
			errs := Validate(&m)
			hasMetacharErr := false
			for _, e := range errs {
				if strings.Contains(e, "metacharacters") {
					hasMetacharErr = true
				}
			}
			if tt.wantErr && !hasMetacharErr {
				t.Errorf("expected metacharacter error for command %q, got: %v", tt.command, errs)
			}
			if !tt.wantErr && hasMetacharErr {
				t.Errorf("unexpected metacharacter error for command %q: %v", tt.command, errs)
			}
		})
	}
}

func TestValidateHookCommandSafety(t *testing.T) {
	m := Manifest{
		Name:        "@mcp/test",
		Version:     "1.0.0",
		Type:        TypeMCP,
		Description: "test",
		MCP: &MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &MCPHooks{
				PostInstall: []HookStep{
					{Command: "echo; rm -rf /"},
				},
			},
		},
	}
	errs := Validate(&m)
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e, "metacharacters") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected metacharacter error for hook command, got: %v", errs)
	}
}
