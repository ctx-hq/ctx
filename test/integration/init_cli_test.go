package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// TestInitCLIManifest verifies that a CLI manifest with new fields can be created and validated.
func TestInitCLIManifest(t *testing.T) {
	m := manifest.Scaffold(manifest.TypeCLI, "test", "fizzy-cli")
	m.Version = "0.1.0"
	m.Description = "CLI for Fizzy"
	m.CLI = &manifest.CLISpec{
		Binary: "fizzy",
		Verify: "fizzy --version",
		Auth:   "Run 'fizzy setup' to configure your API token",
	}
	m.Install = &manifest.InstallSpec{
		Script: "https://raw.githubusercontent.com/basecamp/fizzy-cli/master/scripts/install.sh",
	}
	m.Skill = &manifest.SkillSpec{
		Entry:  "SKILL.md",
		Origin: "native",
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Fatalf("Validate CLI manifest: %v", errs)
	}

	// Verify auth field survives marshal/unmarshal
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	dir := t.TempDir()
	ctxYaml := filepath.Join(dir, "ctx.yaml")
	if err := os.WriteFile(ctxYaml, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	if loaded.Type != manifest.TypeCLI {
		t.Errorf("Type = %q, want cli", loaded.Type)
	}
	if loaded.CLI == nil {
		t.Fatal("CLI is nil after load")
	}
	if loaded.CLI.Binary != "fizzy" {
		t.Errorf("CLI.Binary = %q, want fizzy", loaded.CLI.Binary)
	}
	if loaded.CLI.Auth != "Run 'fizzy setup' to configure your API token" {
		t.Errorf("CLI.Auth = %q, want auth hint", loaded.CLI.Auth)
	}
	if loaded.Install == nil {
		t.Fatal("Install is nil after load")
	}
	if loaded.Install.Script != "https://raw.githubusercontent.com/basecamp/fizzy-cli/master/scripts/install.sh" {
		t.Errorf("Install.Script = %q, want script URL", loaded.Install.Script)
	}
}

// TestInitMCPManifest verifies that an MCP manifest can be created and validated.
func TestInitMCPManifest(t *testing.T) {
	m := manifest.Scaffold(manifest.TypeMCP, "test", "github-mcp")
	m.Version = "1.0.0"
	m.Description = "GitHub MCP server"
	m.MCP = &manifest.MCPSpec{
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "@modelcontextprotocol/server-github"},
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Fatalf("Validate MCP manifest: %v", errs)
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	if loaded.Type != manifest.TypeMCP {
		t.Errorf("Type = %q, want mcp", loaded.Type)
	}
	if loaded.MCP == nil {
		t.Fatal("MCP is nil after load")
	}
	if loaded.MCP.Transport != "stdio" {
		t.Errorf("MCP.Transport = %q, want stdio", loaded.MCP.Transport)
	}
	if loaded.MCP.Command != "npx" {
		t.Errorf("MCP.Command = %q, want npx", loaded.MCP.Command)
	}
	if len(loaded.MCP.Args) != 2 {
		t.Errorf("MCP.Args len = %d, want 2", len(loaded.MCP.Args))
	}
}

// TestInstallSpecGemField verifies the gem field in InstallSpec survives roundtrip.
func TestInstallSpecGemField(t *testing.T) {
	m := manifest.Scaffold(manifest.TypeCLI, "test", "gem-tool")
	m.Version = "1.0.0"
	m.Description = "A Ruby CLI tool"
	m.CLI = &manifest.CLISpec{
		Binary: "gem-tool",
	}
	m.Install = &manifest.InstallSpec{
		Gem: "gem-tool-cli",
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Fatalf("Validate: %v", errs)
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	if loaded.Install == nil {
		t.Fatal("Install is nil after load")
	}
	if loaded.Install.Gem != "gem-tool-cli" {
		t.Errorf("Install.Gem = %q, want gem-tool-cli", loaded.Install.Gem)
	}
}
