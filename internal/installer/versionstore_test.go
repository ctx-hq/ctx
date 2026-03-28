package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwitchCurrent(t *testing.T) {
	dir := t.TempDir()

	// Create version directories
	if err := os.MkdirAll(filepath.Join(dir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "1.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.0.0", "SKILL.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.1.0", "SKILL.md"), []byte("v1.1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Switch to 1.0.0
	if err := SwitchCurrent(dir, "1.0.0"); err != nil {
		t.Fatalf("SwitchCurrent to 1.0.0: %v", err)
	}

	// Verify current points to 1.0.0
	link := filepath.Join(dir, "current")
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if dest != "1.0.0" {
		t.Errorf("current → %q, want %q", dest, "1.0.0")
	}

	// Verify the symlink resolves to the right content
	data, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile through symlink: %v", err)
	}
	if string(data) != "v1" {
		t.Errorf("content = %q, want %q", string(data), "v1")
	}

	// Switch to 1.1.0 (atomic swap)
	if err := SwitchCurrent(dir, "1.1.0"); err != nil {
		t.Fatalf("SwitchCurrent to 1.1.0: %v", err)
	}

	dest, _ = os.Readlink(link)
	if dest != "1.1.0" {
		t.Errorf("after switch: current → %q, want %q", dest, "1.1.0")
	}

	data, _ = os.ReadFile(filepath.Join(link, "SKILL.md"))
	if string(data) != "v1.1" {
		t.Errorf("content after switch = %q, want %q", string(data), "v1.1")
	}
}

func TestSwitchCurrent_NonexistentVersion(t *testing.T) {
	dir := t.TempDir()

	err := SwitchCurrent(dir, "9.9.9")
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestInstalledVersions(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")

	// Create version directories
	if err := os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "1.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create hidden dir (should be ignored)
	if err := os.MkdirAll(filepath.Join(pkgDir, ".tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create current symlink (should be ignored - it's a symlink not dir)
	if err := os.Symlink("2.0.0", filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal(err)
	}

	inst := &Installer{DataDir: dir}
	versions := inst.InstalledVersions("@hong/review")

	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d: %v", len(versions), versions)
	}
	want := []string{"1.0.0", "1.1.0", "2.0.0"}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestInstalledVersions_SemverOrder(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")

	// Create versions that would sort wrong with lexicographic order
	for _, v := range []string{"1.9.0", "1.10.0", "1.2.0", "2.0.0", "1.0.0"} {
		if err := os.MkdirAll(filepath.Join(pkgDir, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	inst := &Installer{DataDir: dir}
	versions := inst.InstalledVersions("@hong/review")

	want := []string{"1.0.0", "1.2.0", "1.9.0", "1.10.0", "2.0.0"}
	if len(versions) != len(want) {
		t.Fatalf("expected %d versions, got %d: %v", len(want), len(versions), versions)
	}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")
	if err := os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("1.0.0", filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal(err)
	}

	inst := &Installer{DataDir: dir}
	ver := inst.CurrentVersion("@hong/review")
	if ver != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", ver, "1.0.0")
	}
}

func TestCurrentVersion_NoSymlink(t *testing.T) {
	dir := t.TempDir()
	inst := &Installer{DataDir: dir}
	ver := inst.CurrentVersion("@nonexistent/pkg")
	if ver != "" {
		t.Errorf("CurrentVersion for missing = %q, want empty", ver)
	}
}

func TestPruneVersions(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")

	// Create 4 versions with content
	for _, v := range []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"} {
		vDir := filepath.Join(pkgDir, v)
		if err := os.MkdirAll(vDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("content-"+v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Set current to 2.0.0
	if err := os.Symlink("2.0.0", filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal(err)
	}

	inst := &Installer{DataDir: dir}

	// Prune keeping 2 versions
	removed, freed, err := inst.PruneVersions("@hong/review", 2)
	if err != nil {
		t.Fatalf("PruneVersions: %v", err)
	}

	// Should remove 1.0.0 and 1.1.0 (oldest two)
	if len(removed) != 2 {
		t.Errorf("removed %d versions, want 2: %v", len(removed), removed)
	}
	if freed <= 0 {
		t.Error("should have freed some bytes")
	}

	// 1.2.0 and 2.0.0 should remain
	remaining := inst.InstalledVersions("@hong/review")
	if len(remaining) != 2 {
		t.Errorf("remaining = %d, want 2: %v", len(remaining), remaining)
	}

	// Current should still work
	ver := inst.CurrentVersion("@hong/review")
	if ver != "2.0.0" {
		t.Errorf("current after prune = %q, want %q", ver, "2.0.0")
	}
}

func TestPruneVersions_KeepsCurrent(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")

	// Create 2 versions, current on older one
	if err := os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("1.0.0", filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal(err)
	}

	inst := &Installer{DataDir: dir}

	// Prune keeping 1 — should keep current (1.0.0) even if it's older
	removed, _, _ := inst.PruneVersions("@hong/review", 1)

	// 2.0.0 is newer but not current, so it should be removed
	if len(removed) != 1 {
		t.Errorf("removed %d, want 1: %v", len(removed), removed)
	}
	if len(removed) > 0 && removed[0] != "2.0.0" {
		t.Errorf("removed %q, want %q", removed[0], "2.0.0")
	}

	// Current should still be intact
	ver := inst.CurrentVersion("@hong/review")
	if ver != "1.0.0" {
		t.Errorf("current = %q, want %q", ver, "1.0.0")
	}
}

func TestPruneVersions_NothingToPrune(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@hong", "review")
	if err := os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("1.0.0", filepath.Join(pkgDir, "current")); err != nil {
		t.Fatal(err)
	}

	inst := &Installer{DataDir: dir}
	removed, _, _ := inst.PruneVersions("@hong/review", 1)

	if len(removed) != 0 {
		t.Errorf("removed %d, want 0 (nothing to prune)", len(removed))
	}
}

func TestPruneVersions_EmptyPackage(t *testing.T) {
	dir := t.TempDir()
	inst := &Installer{DataDir: dir}
	removed, freed, err := inst.PruneVersions("@nonexistent/pkg", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 || freed != 0 {
		t.Error("should be no-op for nonexistent package")
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world!"), 0o644); err != nil {
		t.Fatal(err)
	}

	size := dirSize(dir)
	if size != 11 { // 5 + 6
		t.Errorf("dirSize = %d, want 11", size)
	}
}

func TestIsHidden(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".hidden", true},
		{".ctx-managed", true},
		{"visible", false},
		{"1.0.0", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHidden(tt.name); got != tt.want {
			t.Errorf("isHidden(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
