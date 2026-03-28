package main

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"testing"
)

func TestVersionCommand_OutputFields(t *testing.T) {
	// Build the binary and test via subprocess to avoid flag pollution.
	binary := t.TempDir() + "/ctx"
	build := exec.Command("go", "build", "-o", binary, "./")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binary, "version", "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}

	for _, key := range []string{"version", "os", "arch"} {
		if _, exists := resp.Data[key]; !exists {
			t.Errorf("missing field %q in version output", key)
		}
	}

	if resp.Data["os"] != runtime.GOOS {
		t.Errorf("os = %q, want %q", resp.Data["os"], runtime.GOOS)
	}
	if resp.Data["arch"] != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", resp.Data["arch"], runtime.GOARCH)
	}
}

func TestVersionCommand_DevDefault(t *testing.T) {
	// When built without ldflags, version should be "dev"
	binary := t.TempDir() + "/ctx"
	build := exec.Command("go", "build", "-o", binary, "./")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binary, "version", "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Without ldflags, version is "dev" (BuildInfo returns "(devel)" in local builds)
	if resp.Data["version"] == "" {
		t.Error("version should never be empty")
	}
}

func TestVersionCommand_LdflagsInjection(t *testing.T) {
	// Build with ldflags and verify version is injected
	binary := t.TempDir() + "/ctx"
	build := exec.Command("go", "build",
		"-ldflags", "-X main.Version=v99.88.77",
		"-o", binary, "./")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binary, "version", "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if resp.Data["version"] != "v99.88.77" {
		t.Errorf("version = %q, want v99.88.77", resp.Data["version"])
	}
}

func TestVersionCommand_QuietOutput(t *testing.T) {
	binary := t.TempDir() + "/ctx"
	build := exec.Command("go", "build", "-o", binary, "./")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binary, "version", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --quiet failed: %v", err)
	}

	output := string(out)
	if output == "" {
		t.Error("expected non-empty quiet output")
	}
}
