package adapter

import (
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
