package adapter

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScriptAdapterName(t *testing.T) {
	a := &ScriptAdapter{}
	if a.Name() != "script" {
		t.Errorf("Name() = %q, want %q", a.Name(), "script")
	}
}

func TestScriptAdapterAvailable(t *testing.T) {
	a := &ScriptAdapter{}
	if runtime.GOOS == "windows" {
		if a.Available() {
			t.Error("ScriptAdapter should not be available on Windows")
		}
	} else {
		if !a.Available() {
			t.Error("ScriptAdapter should be available on non-Windows")
		}
	}
}

func TestFindAdapterScript(t *testing.T) {
	spec := InstallSpec{
		Script: "https://example.com/install.sh",
	}
	a, pkg, err := FindAdapter(spec)
	if runtime.GOOS == "windows" {
		if err == nil {
			t.Error("expected error on Windows, got nil")
		}
		return
	}
	if err != nil {
		t.Fatalf("FindAdapter() error: %v", err)
	}
	if a.Name() != "script" {
		t.Errorf("adapter = %q, want %q", a.Name(), "script")
	}
	if pkg != "https://example.com/install.sh" {
		t.Errorf("pkg = %q, want %q", pkg, "https://example.com/install.sh")
	}
}

func TestFindAdapterScriptLowerPriorityThanBrew(t *testing.T) {
	if !commandExists("brew") {
		t.Skip("brew not available")
	}
	spec := InstallSpec{
		Brew:   "some-formula",
		Script: "https://example.com/install.sh",
	}
	a, _, err := FindAdapter(spec)
	if err != nil {
		t.Fatalf("FindAdapter() error: %v", err)
	}
	if a.Name() != "brew" {
		t.Errorf("adapter = %q, want %q (brew should have higher priority than script)", a.Name(), "brew")
	}
}

// --- ScriptAdapter.Uninstall tests ---

func TestScriptAdapterUninstall_RemovesBinary(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "myapp")
	if err := os.WriteFile(binPath, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := &ScriptAdapter{}
	if err := a.Uninstall(context.Background(), binPath); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Error("binary should have been removed")
	}
}

func TestScriptAdapterUninstall_EmptyPath(t *testing.T) {
	a := &ScriptAdapter{}
	err := a.Uninstall(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestScriptAdapterUninstall_ProtectedPaths(t *testing.T) {
	a := &ScriptAdapter{}
	protected := []string{"/usr/bin/ls", "/bin/sh", "/sbin/mount", "/usr/sbin/diskutil"}
	for _, p := range protected {
		err := a.Uninstall(context.Background(), p)
		if err == nil {
			t.Errorf("expected error for protected path %s", p)
		}
	}
}

func TestScriptAdapterUninstall_RelativePath(t *testing.T) {
	a := &ScriptAdapter{}
	err := a.Uninstall(context.Background(), "relative/path/bin")
	if err == nil {
		t.Error("expected error for relative path")
	}
}

func TestScriptAdapterUninstall_NonExistent(t *testing.T) {
	a := &ScriptAdapter{}
	// Should succeed silently (idempotent)
	err := a.Uninstall(context.Background(), "/tmp/does-not-exist-ctx-test")
	if err != nil {
		t.Errorf("Uninstall() should be idempotent for non-existent files, got: %v", err)
	}
}

func TestScriptAdapterUninstall_RefusesDirectory(t *testing.T) {
	dir := t.TempDir()
	a := &ScriptAdapter{}
	err := a.Uninstall(context.Background(), dir)
	if err == nil {
		t.Error("expected error when trying to remove a directory")
	}
}

func TestScriptAdapterUninstall_UserBinDir(t *testing.T) {
	// Simulate ~/.local/bin/myapp — the common case for script-installed binaries
	dir := t.TempDir()
	localBin := filepath.Join(dir, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(localBin, "myapp")
	if err := os.WriteFile(binPath, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := &ScriptAdapter{}
	if err := a.Uninstall(context.Background(), binPath); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Error("binary in ~/.local/bin should have been removed")
	}
}
