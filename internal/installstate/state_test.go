package installstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()

	original := &PackageState{
		FullName:    "@test/my-cli",
		Version:     "1.0.0",
		Type:        "cli",
		InstalledAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CLI: &CLIState{
			Adapter:    "gem",
			AdapterPkg: "fizzy-cli",
			Binary:     "fizzy",
			BinaryPath: "/usr/local/bin/fizzy",
			Verified:   true,
			Status:     "ok",
		},
		Skills: []SkillState{
			{Agent: "claude", SymlinkPath: "~/.claude/skills/my-cli", Status: "ok"},
			{Agent: "cursor", SymlinkPath: "~/.cursor/skills/my-cli", Status: "ok"},
		},
	}

	if err := original.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.SchemaVersion != stateSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, stateSchemaVersion)
	}
	if loaded.FullName != original.FullName {
		t.Errorf("FullName = %q, want %q", loaded.FullName, original.FullName)
	}
	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, original.Type)
	}
	if loaded.CLI == nil {
		t.Fatal("CLI state is nil")
	}
	if loaded.CLI.Adapter != "gem" {
		t.Errorf("CLI.Adapter = %q, want %q", loaded.CLI.Adapter, "gem")
	}
	if loaded.CLI.AdapterPkg != "fizzy-cli" {
		t.Errorf("CLI.AdapterPkg = %q, want %q", loaded.CLI.AdapterPkg, "fizzy-cli")
	}
	if loaded.CLI.Binary != "fizzy" {
		t.Errorf("CLI.Binary = %q, want %q", loaded.CLI.Binary, "fizzy")
	}
	if !loaded.CLI.Verified {
		t.Error("CLI.Verified = false, want true")
	}
	if loaded.CLI.Status != "ok" {
		t.Errorf("CLI.Status = %q, want %q", loaded.CLI.Status, "ok")
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2", len(loaded.Skills))
	}
	if loaded.Skills[0].Agent != "claude" {
		t.Errorf("Skills[0].Agent = %q, want %q", loaded.Skills[0].Agent, "claude")
	}
	if loaded.Skills[1].Agent != "cursor" {
		t.Errorf("Skills[1].Agent = %q, want %q", loaded.Skills[1].Agent, "cursor")
	}
}

func TestStateLoadMissing(t *testing.T) {
	dir := t.TempDir()

	state, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state != nil {
		t.Errorf("Load returned %v, want nil for missing state", state)
	}
}

func TestStateSaveAtomic(t *testing.T) {
	dir := t.TempDir()

	state := &PackageState{
		FullName: "@test/atomic",
		Version:  "1.0.0",
		Type:     "skill",
	}

	if err := state.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the file exists and no temp files remain
	path := filepath.Join(dir, stateFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state.json should exist: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != stateFileName {
			t.Errorf("unexpected file in dir: %s (temp file not cleaned up?)", e.Name())
		}
	}
}

func TestStateRemove(t *testing.T) {
	dir := t.TempDir()

	state := &PackageState{
		FullName: "@test/removable",
		Version:  "1.0.0",
		Type:     "skill",
	}
	if err := state.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Remove(dir); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify file is gone
	path := filepath.Join(dir, stateFileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("state.json should be removed")
	}
}

func TestStateRemoveMissing(t *testing.T) {
	dir := t.TempDir()

	// Removing non-existent state should not error
	if err := Remove(dir); err != nil {
		t.Fatalf("Remove non-existent: %v", err)
	}
}

func TestStateMCPFields(t *testing.T) {
	dir := t.TempDir()

	state := &PackageState{
		FullName: "@test/mcp-server",
		Version:  "2.0.0",
		Type:     "mcp",
		MCP: []MCPState{
			{Agent: "claude", ConfigKey: "github-mcp", Status: "ok"},
		},
	}
	if err := state.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.MCP) != 1 {
		t.Fatalf("MCP len = %d, want 1", len(loaded.MCP))
	}
	if loaded.MCP[0].ConfigKey != "github-mcp" {
		t.Errorf("MCP[0].ConfigKey = %q, want %q", loaded.MCP[0].ConfigKey, "github-mcp")
	}
}
