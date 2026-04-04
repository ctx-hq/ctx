package shellpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupHome creates a temp dir and sets HOME to it, returning the path.
func setupHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Clear vars that might interfere
	t.Setenv("ZDOTDIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CTX_NO_MODIFY_PATH", "")
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("JENKINS_URL", "")
	return home
}

func TestAlreadyInPATH(t *testing.T) {
	home := setupHome(t)
	binDir := filepath.Join(home, ".ctx", "bin")
	t.Setenv("PATH", binDir+":/usr/bin:/bin")
	t.Setenv("SHELL", "/bin/zsh")

	r := ensurePathWith(binDir, "darwin")
	if !r.AlreadyInPATH {
		t.Fatal("expected AlreadyInPATH=true")
	}
	if len(r.Modified) != 0 || r.Err != nil {
		t.Fatal("should not modify or error")
	}
}

func TestAlreadyInPATH_CleanedPath(t *testing.T) {
	home := setupHome(t)
	binDir := filepath.Join(home, ".ctx", "bin")
	// PATH contains the dir with a trailing slash
	t.Setenv("PATH", binDir+"/:/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	r := ensurePathWith(binDir, "darwin")
	if !r.AlreadyInPATH {
		t.Fatal("expected AlreadyInPATH=true with trailing slash in PATH")
	}
}

func TestOptOut(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("CTX_NO_MODIFY_PATH", "1")

	r := ensurePathWith(filepath.Join(home, ".ctx", "bin"), "darwin")
	if r.Skipped != "opt-out" {
		t.Fatalf("expected Skipped=opt-out, got %q", r.Skipped)
	}
}

func TestCIDetection(t *testing.T) {
	ciVars := []struct {
		key, val string
	}{
		{"CI", "true"},
		{"GITHUB_ACTIONS", "true"},
		{"GITLAB_CI", "1"},
		{"CIRCLECI", "1"},
		{"JENKINS_URL", "http://jenkins"},
	}
	for _, tc := range ciVars {
		t.Run(tc.key, func(t *testing.T) {
			home := setupHome(t)
			t.Setenv("PATH", "/usr/bin")
			t.Setenv("SHELL", "/bin/bash")
			t.Setenv(tc.key, tc.val)

			r := ensurePathWith(filepath.Join(home, ".ctx", "bin"), "linux")
			if r.Skipped != "ci" {
				t.Fatalf("expected Skipped=ci for %s, got %q", tc.key, r.Skipped)
			}
		})
	}
}

func TestZsh(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".zshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}

	content, _ := os.ReadFile(expected)
	if !strings.Contains(string(content), binDir) {
		t.Fatal("zshrc should contain binDir")
	}
	if !strings.Contains(string(content), "# ctx") {
		t.Fatal("zshrc should contain # ctx marker")
	}
}

func TestZsh_ZDOTDIR(t *testing.T) {
	home := setupHome(t)
	zdotdir := filepath.Join(home, "custom-zsh")
	if err := os.MkdirAll(zdotdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", zdotdir)

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(zdotdir, ".zshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
}

func TestBash_Darwin(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/bash")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	// On macOS, primary is .bash_profile
	expected := filepath.Join(home, ".bash_profile")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
}

func TestBash_Darwin_BothExist(t *testing.T) {
	home := setupHome(t)
	// Create both files
	writeFile(t, filepath.Join(home, ".bash_profile"), "# existing\n")
	writeFile(t, filepath.Join(home, ".bashrc"), "# existing\n")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/bash")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	// Both should be written
	for _, name := range []string{".bash_profile", ".bashrc"} {
		content, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			t.Fatalf("should be able to read %s: %v", name, err)
		}
		if !strings.Contains(string(content), binDir) {
			t.Fatalf("%s should contain binDir", name)
		}
	}
}

func TestBash_Linux(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/bash")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	// On Linux, primary is .bashrc
	expected := filepath.Join(home, ".bashrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
}

func TestBash_Linux_BothExist(t *testing.T) {
	home := setupHome(t)
	writeFile(t, filepath.Join(home, ".bashrc"), "# existing\n")
	writeFile(t, filepath.Join(home, ".bash_profile"), "# existing\n")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/bash")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	for _, name := range []string{".bashrc", ".bash_profile"} {
		content, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			t.Fatalf("should be able to read %s: %v", name, err)
		}
		if !strings.Contains(string(content), binDir) {
			t.Fatalf("%s should contain binDir", name)
		}
	}
}

func TestFish(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/fish")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".config", "fish", "config.fish")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
	content, _ := os.ReadFile(expected)
	if !strings.Contains(string(content), "fish_add_path") {
		t.Fatal("should use fish_add_path syntax")
	}
}

func TestFish_XDGConfigHome(t *testing.T) {
	home := setupHome(t)
	xdg := filepath.Join(home, "custom-config")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/fish")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(xdg, "fish", "config.fish")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
}

func TestNushell_ConfigExists(t *testing.T) {
	home := setupHome(t)
	nuDir := filepath.Join(home, ".config", "nushell")
	if err := os.MkdirAll(nuDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/nu")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(nuDir, "env.nu")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
	content, _ := os.ReadFile(expected)
	if !strings.Contains(string(content), "split row") {
		t.Fatal("should use nushell syntax")
	}
}

func TestNushell_NoConfig_FallbackProfile(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/nu")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".profile")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected fallback to .profile, got %s", r.ModifiedFile())
	}
}

func TestElvish_ConfigExists(t *testing.T) {
	home := setupHome(t)
	elvDir := filepath.Join(home, ".config", "elvish")
	if err := os.MkdirAll(elvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/elvish")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(elvDir, "rc.elv")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected Modified=%s, got %s", expected, r.ModifiedFile())
	}
	content, _ := os.ReadFile(expected)
	if !strings.Contains(string(content), "set paths") {
		t.Fatal("should use elvish syntax")
	}
}

func TestKsh(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/ksh")

	// No .kshrc exists → fallback to .profile
	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".profile")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .profile, got %s", r.ModifiedFile())
	}
}

func TestKsh_WithKshrc(t *testing.T) {
	home := setupHome(t)
	writeFile(t, filepath.Join(home, ".kshrc"), "# ksh\n")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/ksh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".kshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .kshrc, got %s", r.ModifiedFile())
	}
}

func TestCsh(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/csh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".cshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .cshrc, got %s", r.ModifiedFile())
	}
	content, _ := os.ReadFile(expected)
	if !strings.Contains(string(content), "setenv PATH") {
		t.Fatal("should use csh setenv syntax")
	}
}

func TestTcsh_WithTcshrc(t *testing.T) {
	home := setupHome(t)
	writeFile(t, filepath.Join(home, ".tcshrc"), "# tcsh\n")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/tcsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".tcshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .tcshrc, got %s", r.ModifiedFile())
	}
}

func TestTcsh_NoTcshrc_FallbackCshrc(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/tcsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".cshrc")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .cshrc fallback, got %s", r.ModifiedFile())
	}
}

func TestUnknownShell_FallbackProfile(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/dash")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".profile")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .profile fallback, got %s", r.ModifiedFile())
	}
}

func TestEmptyShell_FallbackProfile(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "linux")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".profile")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected .profile fallback, got %s", r.ModifiedFile())
	}
}

func TestIdempotency(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	binDir := filepath.Join(home, ".ctx", "bin")

	// First call: should write
	r1 := ensurePathWith(binDir, "darwin")
	if r1.Err != nil {
		t.Fatal(r1.Err)
	}
	if len(r1.Modified) == 0 {
		t.Fatal("first call should modify")
	}

	// Second call: PATH env still doesn't include it, but rc file does
	r2 := ensurePathWith(binDir, "darwin")
	if r2.Err != nil {
		t.Fatal(r2.Err)
	}
	if !r2.AlreadyInRC {
		t.Fatal("second call should report AlreadyInRC")
	}
	if len(r2.Modified) != 0 {
		t.Fatal("second call should not modify")
	}

	// Verify only one entry in file
	content, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
	count := strings.Count(string(content), "# ctx")
	if count != 1 {
		t.Fatalf("expected exactly 1 ctx entry, got %d", count)
	}
}

func TestNoTrailingNewline(t *testing.T) {
	home := setupHome(t)
	// Write a file without trailing newline
	rcPath := filepath.Join(home, ".zshrc")
	writeFile(t, rcPath, "existing-content")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	content, _ := os.ReadFile(rcPath)
	// Should have newline before # ctx
	if !strings.Contains(string(content), "existing-content\n# ctx\n") {
		t.Fatalf("should add newline before # ctx, got: %q", string(content))
	}
}

func TestWithTrailingNewline(t *testing.T) {
	home := setupHome(t)
	rcPath := filepath.Join(home, ".zshrc")
	writeFile(t, rcPath, "existing-content\n")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	content, _ := os.ReadFile(rcPath)
	// Should NOT have double newline
	if strings.Contains(string(content), "existing-content\n\n# ctx") {
		t.Fatal("should not add extra newline when file already ends with newline")
	}
}

func TestParentDirCreation(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/usr/bin/fish")

	// ~/.config/fish/ does not exist yet
	binDir := filepath.Join(home, ".ctx", "bin")
	r := ensurePathWith(binDir, "darwin")

	if r.Err != nil {
		t.Fatal(r.Err)
	}
	expected := filepath.Join(home, ".config", "fish", "config.fish")
	if r.ModifiedFile() != expected {
		t.Fatalf("expected %s, got %s", expected, r.ModifiedFile())
	}
	// Verify directory was created
	if !dirExists(filepath.Join(home, ".config", "fish")) {
		t.Fatal("parent directory should have been created")
	}
}

func TestReloadHint(t *testing.T) {
	tests := []struct {
		shell    string
		contains string
	}{
		{"/bin/zsh", "restart your terminal"},
		{"/bin/bash", "restart your terminal"},
		{"/usr/bin/fish", "run: source"},
		{"/bin/csh", "run: source"},
		{"/usr/bin/nu", "restart your terminal"},
		{"/usr/bin/elvish", "restart your terminal"},
	}
	for _, tc := range tests {
		t.Run(filepath.Base(tc.shell), func(t *testing.T) {
			home := setupHome(t)
			t.Setenv("PATH", "/usr/bin")
			t.Setenv("SHELL", tc.shell)

			binDir := filepath.Join(home, ".ctx", "bin")
			r := ensurePathWith(binDir, "darwin")
			if r.Err != nil {
				t.Fatal(r.Err)
			}

			hint := r.ReloadHint()
			if !strings.Contains(hint, tc.contains) {
				t.Fatalf("expected hint to contain %q, got %q", tc.contains, hint)
			}
		})
	}
}

func TestAddToRC_MatchesExactLine(t *testing.T) {
	home := setupHome(t)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/zsh")

	binDir := filepath.Join(home, ".ctx", "bin")
	rcPath := filepath.Join(home, ".zshrc")

	// Write a file that mentions the dir in a comment but does NOT have the export line
	writeFile(t, rcPath, "# previously used: "+binDir+"\n")

	r := ensurePathWith(binDir, "darwin")
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	// Should still write because the exact export line is not present
	if len(r.Modified) == 0 {
		t.Fatal("should modify when dir appears in comment but export line is missing")
	}
	content, _ := os.ReadFile(rcPath)
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)
	if !strings.Contains(string(content), exportLine) {
		t.Fatal("should contain the export line")
	}
}

func TestReloadHint_NoModification(t *testing.T) {
	r := Result{AlreadyInPATH: true}
	if hint := r.ReloadHint(); hint != "" {
		t.Fatalf("expected empty hint, got %q", hint)
	}
}
