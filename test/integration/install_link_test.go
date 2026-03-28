package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
)

// TestInstallRemoveRoundtrip verifies that install → remove leaves no traces.
func TestInstallRemoveRoundtrip(t *testing.T) {
	dataDir := t.TempDir()

	fullName := "@test/roundtrip"
	version := "1.0.0"

	// Simulate install: create version dir + current symlink
	vDir := filepath.Join(dataDir, fullName, version)
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := manifest.Manifest{Name: fullName, Version: version, Type: manifest.TypeSkill}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dataDir, fullName)
	if err := installer.SwitchCurrent(pkgDir, version); err != nil {
		t.Fatal(err)
	}

	// Verify install state
	inst := &installer.Installer{DataDir: dataDir}
	if !inst.IsInstalled(fullName) {
		t.Fatal("package should be installed")
	}

	if _, err := os.Stat(filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal("current symlink should exist")
	}

	// Now simulate remove
	if err := os.RemoveAll(pkgDir); err != nil {
		t.Fatal(err)
	}

	// Verify clean state
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Error("package dir should be gone after remove")
	}

	if inst.IsInstalled(fullName) {
		t.Error("package should not be installed after remove")
	}
}

// TestLinkCleanupRoundtrip verifies that symlinks are properly cleaned on remove.
func TestLinkCleanupRoundtrip(t *testing.T) {
	dir := t.TempDir()

	// Setup: source dir with skill
	srcDir := filepath.Join(dir, "packages", "@test", "skill", "1.0.0")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Setup: multiple "agent" directories
	agentDirs := []string{
		filepath.Join(dir, "claude", "skills"),
		filepath.Join(dir, "cursor", "skills"),
		filepath.Join(dir, "generic", "skills"),
	}
	for _, d := range agentDirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create symlinks (simulating install)
	reg := &installer.LinkRegistry{
		Version: 1,
		Links:   make(map[string][]installer.LinkEntry),
	}

	for i, agentDir := range agentDirs {
		linkPath := filepath.Join(agentDir, "test-skill")
		if err := os.Symlink(srcDir, linkPath); err != nil {
			t.Fatal(err)
		}

		agents := []string{"claude", "cursor", "generic"}
		reg.Add("@test/skill", installer.LinkEntry{
			Agent:  agents[i],
			Type:   installer.LinkSymlink,
			Source: srcDir,
			Target: linkPath,
		})
	}

	// Verify all symlinks exist
	for _, agentDir := range agentDirs {
		linkPath := filepath.Join(agentDir, "test-skill")
		if _, err := os.Lstat(linkPath); err != nil {
			t.Fatalf("symlink should exist at %s", linkPath)
		}
	}

	// Now remove
	entries := reg.Remove("@test/skill")
	cleaned := installer.CleanupLinks(entries)

	if cleaned != 3 {
		t.Errorf("cleaned = %d, want 3", cleaned)
	}

	// Verify all symlinks are gone
	for _, agentDir := range agentDirs {
		linkPath := filepath.Join(agentDir, "test-skill")
		if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
			t.Errorf("symlink should be removed at %s", linkPath)
		}
	}

	// Registry should be empty
	if len(reg.ForPackage("@test/skill")) != 0 {
		t.Error("registry should be empty after cleanup")
	}
}

// TestMultiplePackagesIsolation verifies packages don't interfere with each other.
func TestMultiplePackagesIsolation(t *testing.T) {
	dataDir := t.TempDir()

	packages := []struct {
		name    string
		version string
	}{
		{"@test/pkg-a", "1.0.0"},
		{"@test/pkg-b", "2.0.0"},
		{"@other/pkg-c", "3.0.0"},
	}

	inst := &installer.Installer{DataDir: dataDir}

	// Install all
	for _, pkg := range packages {
		vDir := inst.VersionDir(pkg.name, pkg.version)
		if err := os.MkdirAll(vDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("# "+pkg.name), 0o644); err != nil {
			t.Fatal(err)
		}
		pkgDir := inst.PackageDir(pkg.name)
		if err := installer.SwitchCurrent(pkgDir, pkg.version); err != nil {
			t.Fatal(err)
		}
	}

	// Remove one — others should be unaffected
	if err := os.RemoveAll(inst.PackageDir("@test/pkg-b")); err != nil {
		t.Fatal(err)
	}

	// pkg-a should still work
	data, err := os.ReadFile(filepath.Join(inst.CurrentLink("@test/pkg-a"), "SKILL.md"))
	if err != nil {
		t.Fatalf("pkg-a should be unaffected: %v", err)
	}
	if string(data) != "# @test/pkg-a" {
		t.Errorf("pkg-a content wrong: %q", string(data))
	}

	// pkg-c should still work
	data, err = os.ReadFile(filepath.Join(inst.CurrentLink("@other/pkg-c"), "SKILL.md"))
	if err != nil {
		t.Fatalf("pkg-c should be unaffected: %v", err)
	}
	if string(data) != "# @other/pkg-c" {
		t.Errorf("pkg-c content wrong: %q", string(data))
	}
}
