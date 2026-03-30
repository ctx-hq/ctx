package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
)

// TestInstallStateCreatedOnInstall verifies that state.json is created alongside version dirs.
func TestInstallStateCreatedOnInstall(t *testing.T) {
	dataDir := t.TempDir()

	fullName := "@test/state-pkg"
	version := "1.0.0"
	pkgDir := filepath.Join(dataDir, fullName)
	vDir := filepath.Join(pkgDir, version)

	// Simulate install: create version dir + current symlink
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installer.SwitchCurrent(pkgDir, version); err != nil {
		t.Fatal(err)
	}

	// Save state
	state := &installstate.PackageState{
		FullName: fullName,
		Version:  version,
		Type:     "cli",
		CLI: &installstate.CLIState{
			Adapter:    "gem",
			AdapterPkg: "fizzy-cli",
			Binary:     "fizzy",
			BinaryPath: "/usr/local/bin/fizzy",
			Verified:   true,
			Status:     "ok",
		},
		Skills: []installstate.SkillState{
			{Agent: "claude", SymlinkPath: "/tmp/fake/claude/skills/state-pkg", Status: "ok"},
		},
	}
	if err := state.Save(pkgDir); err != nil {
		t.Fatalf("Save state: %v", err)
	}

	// Verify state.json exists
	statePath := filepath.Join(pkgDir, "state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state.json should exist: %v", err)
	}

	// Verify can load
	loaded, err := installstate.Load(pkgDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CLI == nil {
		t.Fatal("CLI state is nil")
	}
	if loaded.CLI.Adapter != "gem" {
		t.Errorf("CLI.Adapter = %q, want gem", loaded.CLI.Adapter)
	}
	if len(loaded.Skills) != 1 {
		t.Errorf("Skills len = %d, want 1", len(loaded.Skills))
	}
}

// TestInstallStateCleanedOnRemove verifies state.json is removed with the package.
func TestInstallStateCleanedOnRemove(t *testing.T) {
	dataDir := t.TempDir()

	fullName := "@test/clean-state"
	version := "1.0.0"
	pkgDir := filepath.Join(dataDir, fullName)
	vDir := filepath.Join(pkgDir, version)

	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installer.SwitchCurrent(pkgDir, version); err != nil {
		t.Fatal(err)
	}

	state := &installstate.PackageState{
		FullName: fullName,
		Version:  version,
		Type:     "skill",
	}
	if err := state.Save(pkgDir); err != nil {
		t.Fatal(err)
	}

	// Simulate remove — RemoveAll cleans everything including state.json
	if err := os.RemoveAll(pkgDir); err != nil {
		t.Fatal(err)
	}

	// state.json should be gone
	loaded, err := installstate.Load(pkgDir)
	if err != nil {
		t.Fatalf("Load after remove: %v", err)
	}
	if loaded != nil {
		t.Error("state should be nil after package removal")
	}
}

// TestInstallStateRepairBrokenSymlink verifies that a broken symlink can be detected.
func TestInstallStateRepairBrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@test", "repair-test")

	// Create a skill state pointing to a non-existent symlink
	state := &installstate.PackageState{
		FullName: "@test/repair-test",
		Version:  "1.0.0",
		Type:     "skill",
		Skills: []installstate.SkillState{
			{Agent: "claude", SymlinkPath: filepath.Join(dir, "nonexistent"), Status: "ok"},
		},
	}
	if err := state.Save(pkgDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load and verify the broken state is detected
	loaded, err := installstate.Load(pkgDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Check symlink path is broken
	for _, s := range loaded.Skills {
		if _, err := os.Stat(s.SymlinkPath); err == nil {
			t.Errorf("Symlink %s should be broken", s.SymlinkPath)
		}
	}
}
