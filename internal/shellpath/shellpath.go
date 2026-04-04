// Package shellpath provides automatic PATH configuration by writing
// export lines to the user's shell rc file(s).
package shellpath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Result describes the outcome of EnsurePath.
type Result struct {
	AlreadyInPATH bool     // dir was already in $PATH
	AlreadyInRC   bool     // dir was already referenced in the rc file
	Modified      []string // rc files that were modified (empty if none)
	Skipped       string   // reason we skipped: "ci", "opt-out", or ""
	Err           error    // non-nil if we tried to write and failed
}

// ModifiedFile returns the last modified rc file path, or "" if none.
func (r Result) ModifiedFile() string {
	if len(r.Modified) == 0 {
		return ""
	}
	return r.Modified[len(r.Modified)-1]
}

// ReloadHint returns a human-readable hint for reloading the shell.
func (r Result) ReloadHint() string {
	if len(r.Modified) == 0 {
		return ""
	}
	shell := detectShell()
	file := r.ModifiedFile()
	switch shell {
	case "fish":
		return fmt.Sprintf("run: source %s", file)
	case "nu", "nushell", "elvish":
		return "restart your terminal"
	case "csh", "tcsh":
		return fmt.Sprintf("run: source %s", file)
	default:
		return fmt.Sprintf("restart your terminal or run: source %s", file)
	}
}

// EnsurePath checks whether dir is in the current PATH and, if not,
// appends the appropriate export line to the user's shell rc file(s).
// It is idempotent and respects CTX_NO_MODIFY_PATH and CI environments.
func EnsurePath(dir string) Result {
	return ensurePathWith(dir, runtime.GOOS)
}

// ensurePathWith is the testable inner function that accepts goos explicitly.
func ensurePathWith(dir string, goos string) Result {
	// 0. Windows uses a different mechanism (install.ps1 sets user-level PATH).
	if goos == "windows" {
		return Result{Skipped: "windows"}
	}

	// 1. Already in PATH?
	cleanDir := filepath.Clean(dir)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == cleanDir {
			return Result{AlreadyInPATH: true}
		}
	}

	// 2. User opt-out?
	if os.Getenv("CTX_NO_MODIFY_PATH") == "1" {
		return Result{Skipped: "opt-out"}
	}

	// 3. CI environment?
	if isCI() {
		return Result{Skipped: "ci"}
	}

	// 4. Detect shell and resolve rc files
	shell := detectShell()
	home := homeDir()
	entries := resolveRC(shell, dir, home, goos)

	if len(entries) == 0 {
		// Fallback: ~/.profile
		entries = []rcEntry{{
			path: filepath.Join(home, ".profile"),
			line: fmt.Sprintf(`export PATH="%s:$PATH"`, dir),
		}}
	}

	// 5. Write idempotently to each rc file
	var modified []string
	var anyAlreadyInRC bool
	for _, e := range entries {
		written, err := addToRC(e.path, e.line)
		if err != nil {
			return Result{Err: fmt.Errorf("writing %s: %w", e.path, err)}
		}
		if written {
			modified = append(modified, e.path)
		} else {
			anyAlreadyInRC = true
		}
	}

	if len(modified) > 0 {
		return Result{Modified: modified}
	}
	if anyAlreadyInRC {
		return Result{AlreadyInRC: true}
	}
	return Result{}
}

// rcEntry is a resolved rc file path and the line to append.
type rcEntry struct {
	path string
	line string
}

// detectShell returns the basename of $SHELL (e.g., "zsh", "bash", "fish").
func detectShell() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return ""
	}
	return filepath.Base(s)
}

// isCI returns true if running in a known CI environment.
func isCI() bool {
	if os.Getenv("CI") == "true" {
		return true
	}
	for _, key := range []string{"GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "JENKINS_URL"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// resolveRC returns the rc file(s) and export lines for a given shell.
func resolveRC(shell, dir, home, goos string) []rcEntry {
	posixLine := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)

	switch shell {
	case "zsh":
		zdotdir := os.Getenv("ZDOTDIR")
		if zdotdir == "" {
			zdotdir = home
		}
		return []rcEntry{{path: filepath.Join(zdotdir, ".zshrc"), line: posixLine}}

	case "bash":
		return resolveBashRC(dir, home, goos, posixLine)

	case "fish":
		configDir := xdgConfigHome(home)
		return []rcEntry{{
			path: filepath.Join(configDir, "fish", "config.fish"),
			line: fmt.Sprintf("fish_add_path %s", dir),
		}}

	case "nu", "nushell":
		configDir := xdgConfigHome(home)
		nuDir := filepath.Join(configDir, "nushell")
		if dirExists(nuDir) {
			return []rcEntry{{
				path: filepath.Join(nuDir, "env.nu"),
				line: fmt.Sprintf(`$env.PATH = ($env.PATH | split row (char esep) | prepend '%s')`, dir),
			}}
		}
		return []rcEntry{{path: filepath.Join(home, ".profile"), line: posixLine}}

	case "elvish":
		configDir := xdgConfigHome(home)
		elvDir := filepath.Join(configDir, "elvish")
		if dirExists(elvDir) {
			return []rcEntry{{
				path: filepath.Join(elvDir, "rc.elv"),
				line: fmt.Sprintf("set paths = [%s $@paths]", dir),
			}}
		}
		return []rcEntry{{path: filepath.Join(home, ".profile"), line: posixLine}}

	case "ksh", "ksh93":
		kshrc := filepath.Join(home, ".kshrc")
		if fileExists(kshrc) {
			return []rcEntry{{path: kshrc, line: posixLine}}
		}
		return []rcEntry{{path: filepath.Join(home, ".profile"), line: posixLine}}

	case "csh", "tcsh":
		cshLine := fmt.Sprintf("setenv PATH %s:$PATH", dir)
		if shell == "tcsh" {
			tcshrc := filepath.Join(home, ".tcshrc")
			if fileExists(tcshrc) {
				return []rcEntry{{path: tcshrc, line: cshLine}}
			}
		}
		return []rcEntry{{path: filepath.Join(home, ".cshrc"), line: cshLine}}

	default:
		return []rcEntry{{path: filepath.Join(home, ".profile"), line: posixLine}}
	}
}

// resolveBashRC handles macOS vs Linux bash profile selection.
// Matches install.sh semantics: write to whichever rc files already exist.
// If neither exists, create only the platform-preferred file.
func resolveBashRC(dir, home, goos, posixLine string) []rcEntry {
	bashProfile := filepath.Join(home, ".bash_profile")
	bashrc := filepath.Join(home, ".bashrc")

	var entries []rcEntry

	// Preferred order differs by platform
	candidates := []string{bashrc, bashProfile}
	if goos == "darwin" {
		candidates = []string{bashProfile, bashrc}
	}

	// Write to whichever files already exist
	for _, f := range candidates {
		if fileExists(f) {
			entries = append(entries, rcEntry{path: f, line: posixLine})
		}
	}

	// Neither exists — create only the platform-preferred file
	if len(entries) == 0 {
		entries = append(entries, rcEntry{path: candidates[0], line: posixLine})
	}

	return entries
}

// addToRC appends a line to an rc file idempotently.
// Returns (true, nil) if the file was modified, (false, nil) if the line is already present.
func addToRC(rcPath, line string) (bool, error) {
	// Check if the exact export line is already in the file
	content, err := os.ReadFile(rcPath)
	if err == nil && strings.Contains(string(content), line) {
		return false, nil // already present
	}
	// err != nil means file doesn't exist yet — that's fine, we'll create it

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return false, err
	}

	// Build content to append
	var buf strings.Builder
	// If file exists and doesn't end with newline, add one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString("# ctx\n")
	buf.WriteString(line)
	buf.WriteByte('\n')

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}

	if _, err := f.WriteString(buf.String()); err != nil {
		_ = f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	return true, nil
}

func xdgConfigHome(home string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		if h = os.Getenv("HOME"); h == "" {
			h = "/"
		}
	}
	return h
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
