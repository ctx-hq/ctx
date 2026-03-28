package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillBySymlink_Success(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(dir, "skills")
	err := installSkillBySymlink(skillsDir, srcDir, "review")
	if err != nil {
		t.Fatalf("installSkillBySymlink: %v", err)
	}

	// Verify symlink exists
	target := filepath.Join(skillsDir, "review")
	fi, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file/dir")
	}

	// Verify content accessible through symlink
	data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		t.Fatalf("read through symlink: %v", err)
	}
	if string(data) != "# test" {
		t.Errorf("content = %q, want '# test'", string(data))
	}
}

func TestInstallSkillBySymlink_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	srcDir1 := filepath.Join(dir, "v1")
	srcDir2 := filepath.Join(dir, "v2")
	if err := os.MkdirAll(srcDir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir1, "SKILL.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir2, "SKILL.md"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(dir, "skills")

	// Install v1
	if err := installSkillBySymlink(skillsDir, srcDir1, "review"); err != nil {
		t.Fatal(err)
	}

	// Install v2 (should overwrite)
	err := installSkillBySymlink(skillsDir, srcDir2, "review")
	if err != nil {
		t.Fatalf("overwrite failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(skillsDir, "review", "SKILL.md"))
	if string(data) != "v2" {
		t.Errorf("content after overwrite = %q, want 'v2'", string(data))
	}
}

func TestInstallSkillBySymlink_FallbackToCopy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# copy test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test copyDirWithMarker directly (since we can't easily force symlink failure)
	targetDir := filepath.Join(dir, "copied", "review")
	err := copyDirWithMarker(srcDir, targetDir)
	if err != nil {
		t.Fatalf("copyDirWithMarker: %v", err)
	}

	// Verify content copied
	data, err := os.ReadFile(filepath.Join(targetDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "# copy test" {
		t.Errorf("content = %q, want '# copy test'", string(data))
	}

	// Verify .ctx-managed marker
	marker, err := os.ReadFile(filepath.Join(targetDir, ".ctx-managed"))
	if err != nil {
		t.Fatal("missing .ctx-managed marker")
	}
	if len(marker) == 0 {
		t.Error("marker should not be empty")
	}
}

func TestRemoveSkillDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := removeSkillDir(filepath.Join(dir, "skills"), "review")
	if err != nil {
		t.Fatalf("removeSkillDir: %v", err)
	}

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill dir should be removed")
	}
}

func TestRemoveSkillDir_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	// Should not error when dir doesn't exist
	err := removeSkillDir(dir, "nonexistent")
	if err != nil {
		t.Errorf("removing nonexistent should not error: %v", err)
	}
}

func TestCopyDirWithMarker_NestedDirs(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "mid.txt"), []byte("mid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "deep", "leaf.txt"), []byte("leaf"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(dir, "target")
	err := copyDirWithMarker(srcDir, targetDir)
	if err != nil {
		t.Fatalf("copyDirWithMarker nested: %v", err)
	}

	// Verify all files copied
	for _, check := range []struct{ path, content string }{
		{filepath.Join(targetDir, "root.txt"), "root"},
		{filepath.Join(targetDir, "sub", "mid.txt"), "mid"},
		{filepath.Join(targetDir, "sub", "deep", "leaf.txt"), "leaf"},
	} {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Errorf("missing %s: %v", check.path, err)
			continue
		}
		if string(data) != check.content {
			t.Errorf("%s = %q, want %q", check.path, string(data), check.content)
		}
	}
}
