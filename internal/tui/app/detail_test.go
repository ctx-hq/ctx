package app

import (
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/installstate"
)

func TestRenderPkgDetail_ContainsName(t *testing.T) {
	item := pkgItem{
		fullName:    "@hong/review",
		version:     "1.2.0",
		pkgType:     "skill",
		description: "AI-powered code review",
		installed:   true,
	}
	result := renderPkgDetail(item, 40, nil, "")
	if !strings.Contains(result, "@hong/review") {
		t.Errorf("expected detail to contain package name, got: %s", result)
	}
	if !strings.Contains(result, "1.2.0") {
		t.Errorf("expected detail to contain version, got: %s", result)
	}
	if !strings.Contains(result, "browse package") {
		t.Errorf("expected installed package to show browse hint, got: %s", result)
	}
}

func TestRenderPkgDetail_NotInstalled(t *testing.T) {
	item := pkgItem{
		fullName:    "@mcp/github",
		version:     "0.5.0",
		pkgType:     "mcp",
		description: "GitHub MCP server",
		installed:   false,
	}
	result := renderPkgDetail(item, 40, nil, "")
	if !strings.Contains(result, "install command") {
		t.Errorf("expected uninstalled package to show install hint, got: %s", result)
	}
}

func TestRenderAgentDetail_ContainsName(t *testing.T) {
	item := agentItem{
		name:       "Claude Code",
		skillsDir:  "/home/user/.claude/skills",
		skillCount: 5,
	}
	result := renderAgentDetail(item, 40, nil, nil)
	if !strings.Contains(result, "Claude Code") {
		t.Errorf("expected detail to contain agent name, got: %s", result)
	}
	if !strings.Contains(result, "5") {
		t.Errorf("expected detail to contain skill count, got: %s", result)
	}
}

func TestRenderAgentDetail_WithSkills(t *testing.T) {
	item := agentItem{
		name:       "Claude Code",
		skillsDir:  "/home/user/.claude/skills",
		skillCount: 2,
	}
	skills := []AgentSkillEntry{
		{Name: "review", IsSymlink: true, LinkTarget: "/home/data/packages/@hong/review/current"},
		{Name: "local-skill", IsSymlink: false},
	}
	mcpServers := []AgentMCPEntry{
		{Name: "github", Command: "ctx serve @mcp/github"},
	}
	result := renderAgentDetail(item, 60, skills, mcpServers)
	if !strings.Contains(result, "Skills") {
		t.Error("expected Skills section header")
	}
	if !strings.Contains(result, "review") {
		t.Error("expected skill name 'review'")
	}
	if !strings.Contains(result, "local-skill") {
		t.Error("expected skill name 'local-skill'")
	}
	if !strings.Contains(result, "MCP Servers") {
		t.Error("expected MCP Servers section header")
	}
	if !strings.Contains(result, "github") {
		t.Error("expected MCP server name 'github'")
	}
}

func TestRenderAgentDetail_BrowseHint(t *testing.T) {
	item := agentItem{
		name:       "Claude Code",
		skillsDir:  "/home/user/.claude/skills",
		skillCount: 0,
	}
	result := renderAgentDetail(item, 40, nil, nil)
	if !strings.Contains(result, "browse skills") {
		t.Error("expected browse skills hint in agent detail")
	}
}

func TestRenderDoctorDetail_Pass(t *testing.T) {
	item := doctorItem{
		name:   "Version check",
		status: "pass",
		detail: "v0.20.0 is latest",
	}
	result := renderDoctorDetail(item, 40)
	if !strings.Contains(result, "Version check") {
		t.Errorf("expected detail to contain check name, got: %s", result)
	}
	if !strings.Contains(result, "v0.20.0") {
		t.Errorf("expected detail to contain detail text, got: %s", result)
	}
}

func TestRenderDoctorDetail_WithHint(t *testing.T) {
	item := doctorItem{
		name:   "Auth",
		status: "warn",
		detail: "Not authenticated",
		hint:   "Run ctx auth login",
	}
	result := renderDoctorDetail(item, 40)
	if !strings.Contains(result, "Hint") {
		t.Errorf("expected detail to contain hint section, got: %s", result)
	}
	if !strings.Contains(result, "ctx auth login") {
		t.Errorf("expected detail to contain hint text, got: %s", result)
	}
}

func TestRenderPkgDetail_WithState(t *testing.T) {
	item := pkgItem{
		fullName: "@hong/review", version: "1.2.0", pkgType: "skill",
		description: "Code review", installed: true,
	}
	state := &installstate.PackageState{
		Skills: []installstate.SkillState{
			{Agent: "claude", SymlinkPath: "/home/.claude/skills/review", Status: "ok"},
			{Agent: "cursor", SymlinkPath: "/home/.cursor/skills/review", Status: "broken"},
		},
	}
	result := renderPkgDetail(item, 60, state, "")
	if !strings.Contains(result, "claude") {
		t.Error("expected agent name 'claude' in detail")
	}
	if !strings.Contains(result, "cursor") {
		t.Error("expected agent name 'cursor' in detail")
	}
	if !strings.Contains(result, "Linked Agents") {
		t.Error("expected 'Linked Agents' section header")
	}
}

func TestRenderPkgDetail_WithSkillContent(t *testing.T) {
	item := pkgItem{
		fullName: "@hong/review", version: "1.2.0", pkgType: "skill",
		description: "Code review", installed: true,
	}
	result := renderPkgDetail(item, 60, nil, "# Hello\n\nThis is a skill.")
	if !strings.Contains(result, "Documentation") {
		t.Error("expected 'Documentation' section header when skillContent is provided")
	}
	// Glamour should render the markdown (or fallback to raw text).
	if !strings.Contains(result, "Hello") {
		t.Error("expected rendered markdown to contain 'Hello'")
	}
}

func TestRenderEmptyDetail_Modes(t *testing.T) {
	tests := []struct {
		mode     viewMode
		contains string
	}{
		{modeInstalled, "No package selected"},
		{modeSearch, "search"},
		{modeAgents, "No agents detected"},
		{modeDoctor, "diagnostics"},
		{modeBrowse, "Select a file"},
	}
	for _, tt := range tests {
		result := renderEmptyDetail(tt.mode, 40)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("mode %d: expected %q in %q", tt.mode, tt.contains, result)
		}
	}
}

func TestRenderFileContent_Markdown(t *testing.T) {
	result := renderFileContent("README.md", "# Title\n\nSome text.", 60)
	if !strings.Contains(result, "Title") {
		t.Errorf("expected markdown rendering to contain 'Title', got: %s", result)
	}
}

func TestRenderFileContent_GoCode(t *testing.T) {
	result := renderFileContent("main.go", "package main\n\nfunc main() {}\n", 60)
	if !strings.Contains(result, "main") {
		t.Errorf("expected code rendering to contain 'main', got: %s", result)
	}
}

func TestRenderFileContent_PlainText(t *testing.T) {
	result := renderFileContent("data.bin", "hello world", 60)
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected plain text fallback, got: %s", result)
	}
}

func TestLangFromExt(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"main.go", "go"},
		{"script.sh", "bash"},
		{"app.py", "python"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.json", "json"},
		{"config.toml", "toml"},
		{"README.md", "markdown"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		got := langFromExt(tt.name)
		if got != tt.want {
			t.Errorf("langFromExt(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
