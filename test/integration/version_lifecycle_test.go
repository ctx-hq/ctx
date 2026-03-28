package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/installer"
)

// TestVersionLifecycle tests: install v1 → install v2 → use v1 → prune
func TestVersionLifecycle(t *testing.T) {
	dataDir := t.TempDir()

	inst := &installer.Installer{
		DataDir: dataDir,
	}

	fullName := "@test/lifecycle"

	// Simulate installing v1.0.0
	v1Dir := inst.VersionDir(fullName, "1.0.0")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "SKILL.md"), []byte("# v1.0.0 content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "manifest.json"), []byte(`{"name":"@test/lifecycle","version":"1.0.0","type":"skill"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := inst.PackageDir(fullName)
	if err := installer.SwitchCurrent(pkgDir, "1.0.0"); err != nil {
		t.Fatalf("SwitchCurrent to 1.0.0: %v", err)
	}

	// Verify current = 1.0.0
	if v := inst.CurrentVersion(fullName); v != "1.0.0" {
		t.Fatalf("current = %q, want 1.0.0", v)
	}

	// Verify reading through current/ symlink works
	data, err := os.ReadFile(filepath.Join(inst.CurrentLink(fullName), "SKILL.md"))
	if err != nil {
		t.Fatalf("read through current: %v", err)
	}
	if string(data) != "# v1.0.0 content" {
		t.Errorf("content = %q, want v1 content", string(data))
	}

	// Simulate installing v2.0.0
	v2Dir := inst.VersionDir(fullName, "2.0.0")
	if err := os.MkdirAll(v2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "SKILL.md"), []byte("# v2.0.0 content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "manifest.json"), []byte(`{"name":"@test/lifecycle","version":"2.0.0","type":"skill"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installer.SwitchCurrent(pkgDir, "2.0.0"); err != nil {
		t.Fatalf("SwitchCurrent to 2.0.0: %v", err)
	}

	// Both versions should exist
	versions := inst.InstalledVersions(fullName)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}

	// Current should be 2.0.0
	if v := inst.CurrentVersion(fullName); v != "2.0.0" {
		t.Fatalf("current = %q, want 2.0.0", v)
	}

	// Read through current should get v2 content
	data, _ = os.ReadFile(filepath.Join(inst.CurrentLink(fullName), "SKILL.md"))
	if string(data) != "# v2.0.0 content" {
		t.Errorf("after upgrade: content = %q, want v2 content", string(data))
	}

	// Switch back to v1 (ctx use)
	if err := installer.SwitchCurrent(pkgDir, "1.0.0"); err != nil {
		t.Fatalf("SwitchCurrent back to 1.0.0: %v", err)
	}

	// Current should be 1.0.0 again
	if v := inst.CurrentVersion(fullName); v != "1.0.0" {
		t.Fatalf("after rollback: current = %q, want 1.0.0", v)
	}
	data, _ = os.ReadFile(filepath.Join(inst.CurrentLink(fullName), "SKILL.md"))
	if string(data) != "# v1.0.0 content" {
		t.Errorf("after rollback: content = %q, want v1 content", string(data))
	}

	// Prune — should remove v2 since current is v1
	removed, freed, err := inst.PruneVersions(fullName, 1)
	if err != nil {
		t.Fatalf("PruneVersions: %v", err)
	}
	if len(removed) != 1 || removed[0] != "2.0.0" {
		t.Errorf("removed = %v, want [2.0.0]", removed)
	}
	if freed <= 0 {
		t.Error("should have freed bytes")
	}

	// Only v1.0.0 remains
	versions = inst.InstalledVersions(fullName)
	if len(versions) != 1 || versions[0] != "1.0.0" {
		t.Errorf("after prune: versions = %v, want [1.0.0]", versions)
	}
}

// TestLinkRegistryRoundtrip tests: add links → verify → remove → verify clean
func TestLinkRegistryRoundtrip(t *testing.T) {
	dir := t.TempDir()

	// Create a source skill dir
	srcDir := filepath.Join(dir, "source", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(srcDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcDir, []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create agent dirs
	agentSkillDir := filepath.Join(dir, "agent-claude", "skills")
	if err := os.MkdirAll(agentSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create symlink (simulating install)
	linkTarget := filepath.Join(agentSkillDir, "review")
	if err := os.Symlink(filepath.Dir(srcDir), linkTarget); err != nil {
		t.Fatal(err)
	}

	// Register in link registry
	reg := &installer.LinkRegistry{
		Version: 1,
		Links:   make(map[string][]installer.LinkEntry),
	}
	reg.Add("@hong/review", installer.LinkEntry{
		Agent:  "claude",
		Type:   installer.LinkSymlink,
		Source: filepath.Dir(srcDir),
		Target: linkTarget,
	})

	// Verify — should be healthy
	issues := reg.Verify()
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %v", len(issues), issues)
	}

	// Remove and cleanup
	entries := reg.Remove("@hong/review")
	cleaned := installer.CleanupLinks(entries)
	if cleaned != 1 {
		t.Errorf("cleaned = %d, want 1", cleaned)
	}

	// Verify symlink is gone
	if _, err := os.Lstat(linkTarget); !os.IsNotExist(err) {
		t.Error("symlink should be removed after cleanup")
	}

	// Registry should be empty
	if len(reg.ForPackage("@hong/review")) != 0 {
		t.Error("registry should be empty after remove")
	}
}

// TestCurrentSymlinkChain tests the two-level symlink pattern:
// agent dir → current → version dir
func TestCurrentSymlinkChain(t *testing.T) {
	dir := t.TempDir()

	// Setup: package with version and current
	pkgDir := filepath.Join(dir, "packages", "@hong", "review")
	v1Dir := filepath.Join(pkgDir, "1.0.0")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "SKILL.md"), []byte("# skill content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installer.SwitchCurrent(pkgDir, "1.0.0"); err != nil {
		t.Fatal(err)
	}

	// Simulate agent symlink pointing to current/
	agentDir := filepath.Join(dir, "agent", "skills")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	agentLink := filepath.Join(agentDir, "review")
	currentDir := filepath.Join(pkgDir, "current")
	if err := os.Symlink(currentDir, agentLink); err != nil {
		t.Fatal(err)
	}

	// Read through the two-level chain: agentLink → current → 1.0.0/SKILL.md
	data, err := os.ReadFile(filepath.Join(agentLink, "SKILL.md"))
	if err != nil {
		t.Fatalf("read through chain: %v", err)
	}
	if string(data) != "# skill content" {
		t.Errorf("content = %q, want '# skill content'", string(data))
	}

	// Now switch version — agent link should automatically resolve to new content
	v2Dir := filepath.Join(pkgDir, "2.0.0")
	if err := os.MkdirAll(v2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "SKILL.md"), []byte("# v2 content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installer.SwitchCurrent(pkgDir, "2.0.0"); err != nil {
		t.Fatal(err)
	}

	// Same agent link now resolves to v2
	data, err = os.ReadFile(filepath.Join(agentLink, "SKILL.md"))
	if err != nil {
		t.Fatalf("read after switch: %v", err)
	}
	if string(data) != "# v2 content" {
		t.Errorf("after switch: content = %q, want '# v2 content'", string(data))
	}
}
