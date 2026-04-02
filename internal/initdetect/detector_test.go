package initdetect

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestParseSource(t *testing.T) {
	tests := []struct {
		input    string
		wantKind SourceKind
		wantKey  string
	}{
		{"npm:@playwright/mcp", SourceNPM, "@playwright/mcp"},
		{"npm:lodash", SourceNPM, "lodash"},
		{"github:github/github-mcp-server", SourceGitHub, "github/github-mcp-server"},
		{"github:microsoft/playwright-mcp", SourceGitHub, "microsoft/playwright-mcp"},
		{"docker:ghcr.io/github/github-mcp-server", SourceDocker, "ghcr.io/github/github-mcp-server"},
		{"docker:ghcr.io/org/image:v1.0", SourceDocker, "ghcr.io/org/image:v1.0"},
		{"/Users/hong/project", SourceLocal, "/Users/hong/project"},
		{"./my-project", SourceLocal, "./my-project"},
		// Inferred GitHub (owner/repo pattern without prefix)
		{"microsoft/playwright-mcp", SourceGitHub, "microsoft/playwright-mcp"},
	}

	for _, tt := range tests {
		kind, key := ParseSource(tt.input)
		if kind != tt.wantKind {
			t.Errorf("ParseSource(%q) kind = %q, want %q", tt.input, kind, tt.wantKind)
		}
		if key != tt.wantKey {
			t.Errorf("ParseSource(%q) key = %q, want %q", tt.input, key, tt.wantKey)
		}
	}
}

func TestInferCtxName(t *testing.T) {
	tests := []struct {
		name    string
		result  DetectResult
		want    string
	}{
		{
			name:   "npm MCP package",
			result: DetectResult{PackageType: manifest.TypeMCP, Name: "@playwright/mcp"},
			want:   "@mcp/playwright",
		},
		{
			name:   "GitHub MCP server with suffix",
			result: DetectResult{PackageType: manifest.TypeMCP, Name: "github-mcp-server"},
			want:   "@mcp/github",
		},
		{
			name:   "MCP with mcp- prefix",
			result: DetectResult{PackageType: manifest.TypeMCP, Name: "mcp-server-puppeteer"},
			want:   "@mcp/puppeteer",
		},
		{
			name:   "CLI package",
			result: DetectResult{PackageType: manifest.TypeCLI, Name: "ripgrep"},
			want:   "@cli/ripgrep",
		},
		{
			name:   "Skill package",
			result: DetectResult{PackageType: manifest.TypeSkill, Name: "my-skill"},
			want:   "@community/my-skill",
		},
		{
			name:   "plain name no scope",
			result: DetectResult{PackageType: manifest.TypeMCP, Name: "playwright"},
			want:   "@mcp/playwright",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferCtxName(&tt.result)
			if got != tt.want {
				t.Errorf("inferCtxName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToManifest_MCP(t *testing.T) {
	r := &DetectResult{
		Kind:        SourceNPM,
		PackageType: manifest.TypeMCP,
		Name:        "playwright",
		Version:     "0.0.70",
		Description: "Browser automation",
		License:     "Apache-2.0",
		Upstream:    &manifest.UpstreamSpec{NPM: "@playwright/mcp", Tracking: "npm"},
		MCP: &MCPDetection{
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", "@playwright/mcp@0.0.70"},
			Require:   &manifest.MCPRequireSpec{Bins: []string{"node"}},
		},
	}

	m := ToManifest(r)

	if m.Type != manifest.TypeMCP {
		t.Errorf("Type = %q, want mcp", m.Type)
	}
	if m.MCP == nil {
		t.Fatal("MCP is nil")
	}
	if m.MCP.Command != "npx" {
		t.Errorf("MCP.Command = %q, want npx", m.MCP.Command)
	}
	if m.MCP.Require == nil || len(m.MCP.Require.Bins) != 1 {
		t.Errorf("MCP.Require.Bins = %v", m.MCP.Require)
	}
	if m.Upstream == nil || m.Upstream.NPM != "@playwright/mcp" {
		t.Errorf("Upstream = %v", m.Upstream)
	}

	// Should pass validation (name format may not match @scope/name but that's ok for generation)
	errs := manifest.Validate(m)
	if len(errs) != 0 {
		t.Errorf("Validate() errors: %v", errs)
	}
}

func TestToManifest_CLI(t *testing.T) {
	r := &DetectResult{
		Kind:        SourceNPM,
		PackageType: manifest.TypeCLI,
		Name:        "ripgrep",
		Version:     "14.1.0",
		Description: "Fast search tool",
		CLI: &CLIDetection{
			Binary: "rg",
			Verify: "rg --version",
			Install: &manifest.InstallSpec{
				Npm: "ripgrep",
			},
		},
	}

	m := ToManifest(r)

	if m.Type != manifest.TypeCLI {
		t.Errorf("Type = %q, want cli", m.Type)
	}
	if m.CLI == nil || m.CLI.Binary != "rg" {
		t.Errorf("CLI.Binary = %v", m.CLI)
	}
	if m.Skill == nil || m.Skill.Origin != "wrapped" {
		t.Errorf("Skill.Origin = %v", m.Skill)
	}
	if m.Install == nil || m.Install.Npm != "ripgrep" {
		t.Errorf("Install = %v", m.Install)
	}
}

func TestDetectDocker(t *testing.T) {
	tests := []struct {
		image       string
		wantName    string
		wantVersion string
	}{
		{"ghcr.io/github/github-mcp-server:v0.2.0", "github-mcp-server", "0.2.0"},
		{"ghcr.io/github/github-mcp-server", "github-mcp-server", "latest"},
		{"myregistry.io/org/tool:1.0", "tool", "1.0"},
	}

	for _, tt := range tests {
		r, err := detectDocker(nil, tt.image)
		if err != nil {
			t.Fatalf("detectDocker(%q) error: %v", tt.image, err)
		}
		if r.Name != tt.wantName {
			t.Errorf("detectDocker(%q).Name = %q, want %q", tt.image, r.Name, tt.wantName)
		}
		if r.Version != tt.wantVersion {
			t.Errorf("detectDocker(%q).Version = %q, want %q", tt.image, r.Version, tt.wantVersion)
		}
		if r.MCP == nil || r.MCP.Command != "docker" {
			t.Errorf("detectDocker(%q).MCP.Command != docker", tt.image)
		}
	}
}

func TestExtractMinVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{">=18", "18.0.0"},
		{">=18.0.0", "18.0.0"},
		{">=20.0.0", "20.0.0"},
		{"^18", ""},          // not >= format
		{"18.0.0", ""},       // no prefix
		{"", ""},
	}

	for _, tt := range tests {
		got := extractMinVersion(tt.input)
		if got != tt.want {
			t.Errorf("extractMinVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServerJSONParse(t *testing.T) {
	data := []byte(`{
		"name": "io.github.test/server",
		"description": "Test MCP server",
		"version": "1.0.0",
		"packages": [
			{
				"registryType": "npm",
				"transport": {"type": "stdio"},
				"command": "node",
				"args": ["dist/index.js"]
			}
		],
		"remotes": [
			{
				"type": "streamable-http",
				"url": "https://api.example.com/mcp/"
			}
		],
		"env": [
			{
				"name": "API_KEY",
				"description": "API key for authentication",
				"isRequired": true,
				"isSecret": true
			}
		]
	}`)

	sj, err := ParseServerJSONBytes(data)
	if err != nil {
		t.Fatalf("ParseServerJSONBytes() error: %v", err)
	}

	if sj.Name != "io.github.test/server" {
		t.Errorf("Name = %q", sj.Name)
	}
	if len(sj.Packages) != 1 {
		t.Fatalf("Packages count = %d", len(sj.Packages))
	}
	if sj.Packages[0].Transport.Type != "stdio" {
		t.Errorf("Package transport = %q", sj.Packages[0].Transport.Type)
	}
	if len(sj.Remotes) != 1 || sj.Remotes[0].URL != "https://api.example.com/mcp/" {
		t.Errorf("Remotes = %v", sj.Remotes)
	}
	if len(sj.Env) != 1 || sj.Env[0].Name != "API_KEY" || !sj.Env[0].Required {
		t.Errorf("Env = %v", sj.Env)
	}

	// Test applyServerJSON
	mcp := &MCPDetection{Transport: "stdio"}
	applyServerJSON(mcp, sj)

	if mcp.Command != "node" {
		t.Errorf("Applied command = %q, want node", mcp.Command)
	}
	if len(mcp.Env) != 1 || mcp.Env[0].Name != "API_KEY" {
		t.Errorf("Applied env = %v", mcp.Env)
	}
	if len(mcp.Transports) != 1 || mcp.Transports[0].URL != "https://api.example.com/mcp/" {
		t.Errorf("Applied transports = %v", mcp.Transports)
	}
}

func TestDetectLocal(t *testing.T) {
	// Test with the playwright-mcp fixture (has server.json analog in refs)
	// Use the mcp_reference fixture which is a local directory
	r, err := detectLocal(nil, "../../test/fixtures/mcp_reference")
	if err != nil {
		t.Fatalf("detectLocal() error: %v", err)
	}
	if r == nil {
		t.Fatal("detectLocal() returned nil")
	}
	// The fixture only has ctx.yaml, which will be detected based on type detection heuristics
	// The name should be derived from directory name
	if r.Name != "mcp_reference" {
		t.Errorf("Name = %q, want mcp_reference", r.Name)
	}
}
