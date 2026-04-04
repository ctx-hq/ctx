package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"ssh github", "git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"ssh gitlab", "git@gitlab.com:group/repo.git", "https://gitlab.com/group/repo"},
		{"ssh no .git", "git@github.com:owner/repo", "https://github.com/owner/repo"},
		{"https with .git", "https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"https clean", "https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"https trailing slash", "https://github.com/owner/repo/", "https://github.com/owner/repo"},
		{"ssh:// scheme", "ssh://git@github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"ssh:// with port", "ssh://git@github.com:22/owner/repo.git", "https://github.com/owner/repo"},
		{"empty", "", ""},
		{"whitespace", "  ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitURL(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeGitURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRemoteURL_NoGitDir(t *testing.T) {
	dir := t.TempDir()
	got := RemoteURL(dir)
	if got != "" {
		t.Errorf("RemoteURL(non-git dir) = %q, want empty", got)
	}
}

func TestRemoteURL_WithOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "git@github.com:test/repo.git")

	got := RemoteURL(dir)
	want := "https://github.com/test/repo"
	if got != want {
		t.Errorf("RemoteURL() = %q, want %q", got, want)
	}
}

func TestRemoteURL_NoRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)

	got := RemoteURL(dir)
	if got != "" {
		t.Errorf("RemoteURL(no remote) = %q, want empty", got)
	}
}

func TestAuthor_NoGitDir(t *testing.T) {
	// git config user.name may return the global config even without .git.
	// That's acceptable — we just verify it doesn't crash.
	dir := t.TempDir()
	_ = Author(dir) // should not panic
}

func TestAuthor_Configured(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)
	runGit(t, dir, "config", "user.name", "Test User")

	got := Author(dir)
	if got != "Test User" {
		t.Errorf("Author() = %q, want %q", got, "Test User")
	}
}

func TestChangedDirs_WithChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create initial commit with a skill
	skillDir := filepath.Join(dir, "skills", "translate")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add translate skill")

	// Modify the skill in a new commit
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "update translate")

	got := ChangedDirs(dir, "HEAD~1", []string{"skills/translate"})
	if len(got) != 1 {
		t.Fatalf("ChangedDirs = %v, want 1 entry", got)
	}
	if got[0] != "skills/translate" {
		t.Errorf("ChangedDirs[0] = %q, want %q", got[0], "skills/translate")
	}
}

func TestChangedDirs_NoChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)

	got := ChangedDirs(dir, "HEAD~1", nil)
	if len(got) != 0 {
		t.Errorf("ChangedDirs = %v, want empty", got)
	}
}

func TestChangedDirs_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	got := ChangedDirs(dir, "HEAD~1", nil)
	if got != nil {
		t.Errorf("ChangedDirs(non-git) = %v, want nil", got)
	}
}

func TestChangedDirs_PrefixFilter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create files in skills/ and docs/
	for _, p := range []string{"skills/a/SKILL.md", "docs/readme.md"} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add files")

	// Modify both
	for _, p := range []string{"skills/a/SKILL.md", "docs/readme.md"} {
		if err := os.WriteFile(filepath.Join(dir, p), []byte("updated"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "update both")

	// Only skills/a member dir
	got := ChangedDirs(dir, "HEAD~1", []string{"skills/a"})
	if len(got) != 1 {
		t.Fatalf("ChangedDirs(skills/a) = %v, want 1 entry", got)
	}
	if got[0] != "skills/a" {
		t.Errorf("got %q, want %q", got[0], "skills/a")
	}
}

// initGitRepo creates a bare-minimum git repo in dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	// Set required config to avoid warnings
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "test")
	// Create initial commit so the repo is valid
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init", "--allow-empty")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
