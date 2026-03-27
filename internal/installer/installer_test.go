package installer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.lock")

	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	// Add entries
	lf.Add(LockEntry{
		FullName:    "@hong/my-skill",
		Version:     "1.0.0",
		Type:        "skill",
		Source:      "registry",
		InstallPath: "/tmp/test",
	})
	lf.Add(LockEntry{
		FullName:    "@mcp/github",
		Version:     "2.1.0",
		Type:        "mcp",
		Source:      "registry",
		InstallPath: "/tmp/test2",
	})

	// Save
	if err := lf.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}

	// Load
	lf2, err := LoadLockFile(path)
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}

	if len(lf2.Packages) != 2 {
		t.Errorf("Packages count = %d, want 2", len(lf2.Packages))
	}

	if !lf2.Has("@hong/my-skill") {
		t.Error("missing @hong/my-skill")
	}
	if !lf2.Has("@mcp/github") {
		t.Error("missing @mcp/github")
	}

	entry, _ := lf2.Get("@hong/my-skill")
	if entry.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.0.0")
	}
}

func TestLockFileAddUpdate(t *testing.T) {
	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	lf.Add(LockEntry{
		FullName: "@test/pkg",
		Version:  "1.0.0",
	})

	first := lf.Packages["@test/pkg"].InstalledAt

	// Small delay to ensure time difference
	time.Sleep(time.Millisecond)

	lf.Add(LockEntry{
		FullName: "@test/pkg",
		Version:  "1.1.0",
	})

	entry := lf.Packages["@test/pkg"]
	if entry.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.1.0")
	}
	if entry.InstalledAt != first {
		t.Error("InstalledAt should be preserved on update")
	}
}

func TestLockFileRemove(t *testing.T) {
	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	lf.Add(LockEntry{FullName: "@test/a", Version: "1.0.0"})
	lf.Add(LockEntry{FullName: "@test/b", Version: "1.0.0"})

	lf.Remove("@test/a")

	if lf.Has("@test/a") {
		t.Error("@test/a should be removed")
	}
	if !lf.Has("@test/b") {
		t.Error("@test/b should still exist")
	}
}

func TestLoadLockFileNotExist(t *testing.T) {
	lf, err := LoadLockFile("/nonexistent/ctx.lock")
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}
	if len(lf.Packages) != 0 {
		t.Errorf("expected empty lockfile, got %d packages", len(lf.Packages))
	}
}
