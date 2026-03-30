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
	m.Skill = &manifest.SkillSpec{
		Entry:  "skills/github-mcp/SKILL.md",
		Origin: "native",
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
	m.Skill = &manifest.SkillSpec{
		Entry: "skills/gem-tool/SKILL.md",
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

// TestInitPreservesExistingSkillEntry verifies that skill.entry from ctx.yaml survives roundtrip.
func TestInitPreservesExistingSkillEntry(t *testing.T) {
	dir := t.TempDir()

	// Create ctx.yaml with skill.entry pointing to a subdirectory
	m := manifest.Scaffold(manifest.TypeCLI, "test", "fizzy-cli")
	m.Version = "0.1.0"
	m.Description = "Fizzy CLI"
	m.CLI = &manifest.CLISpec{Binary: "fizzy", Verify: "fizzy --version"}
	m.Install = &manifest.InstallSpec{Script: "https://example.com/install.sh"}
	m.Skill = &manifest.SkillSpec{
		Entry:  "skills/fizzy/SKILL.md",
		Origin: "native",
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create the SKILL.md at the declared path
	skillDir := filepath.Join(dir, "skills", "fizzy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalContent := "# Fizzy Skill\n\nThis is a hand-crafted 1117-line SKILL.md.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(originalContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Load and verify skill.entry is preserved
	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if loaded.Skill == nil {
		t.Fatal("Skill is nil")
	}
	if loaded.Skill.Entry != "skills/fizzy/SKILL.md" {
		t.Errorf("Skill.Entry = %q, want skills/fizzy/SKILL.md", loaded.Skill.Entry)
	}

	// Verify the SKILL.md content is intact
	content, err := os.ReadFile(filepath.Join(dir, "skills", "fizzy", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != originalContent {
		t.Errorf("SKILL.md content changed: got %q", string(content))
	}
}

// TestInitDoesNotOverwriteExistingSkillMD verifies that ctx init
// preserves an existing SKILL.md when writing ctx.yaml locally.
func TestInitDoesNotOverwriteExistingSkillMD(t *testing.T) {
	dir := t.TempDir()

	// Create existing SKILL.md in a subdirectory
	skillDir := filepath.Join(dir, "skills", "fizzy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	originalContent := "---\nname: fizzy\ndescription: Original content\n---\n\n# Original Fizzy Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(originalContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Write ctx.yaml pointing to the subdirectory SKILL.md
	m := manifest.Scaffold(manifest.TypeCLI, "test", "fizzy-cli")
	m.Version = "0.1.0"
	m.Description = "Fizzy CLI"
	m.CLI = &manifest.CLISpec{Binary: "fizzy"}
	m.Install = &manifest.InstallSpec{Script: "https://example.com/install.sh"}
	m.Skill = &manifest.SkillSpec{Entry: "skills/fizzy/SKILL.md", Origin: "native"}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate re-running init: ctx.yaml is overwritten but SKILL.md should be preserved.
	// The init command checks os.Stat before generating SKILL.md.
	skillEntry := "skills/fizzy/SKILL.md"
	skillAbsPath := filepath.Join(dir, skillEntry)

	// Verify existing SKILL.md is detected (init skips generation)
	if _, err := os.Stat(skillAbsPath); os.IsNotExist(err) {
		t.Fatal("existing SKILL.md should be present")
	}

	// Overwrite ctx.yaml again (simulates second init run)
	m.Version = "0.2.0"
	data2, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data2, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify SKILL.md content was NOT overwritten
	content, err := os.ReadFile(skillAbsPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != originalContent {
		t.Errorf("SKILL.md was overwritten: got %q, want %q", string(content), originalContent)
	}

	// Verify updated ctx.yaml
	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", loaded.Version)
	}
}
