package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/introspect"
	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestWrapGeneratesValidManifest(t *testing.T) {
	// Use "go" as a reliable test binary
	binary := "go"

	helpText, err := introspect.CaptureHelp(binary)
	if err != nil {
		t.Fatalf("CaptureHelp(%s) error: %v", binary, err)
	}

	vr := introspect.CaptureVersion(binary)
	installSpec := introspect.DetectInstallMethod(binary)

	m := &manifest.Manifest{
		Name:        "@test/go",
		Version:     vr.Version,
		Type:        manifest.TypeCLI,
		Description: "Go programming language",
		CLI: &manifest.CLISpec{
			Binary: binary,
			Verify: "go version",
		},
		Skill: &manifest.SkillSpec{
			Entry:  "SKILL.md",
			Origin: "wrapped",
		},
		Install: installSpec,
	}

	errs := manifest.Validate(m)
	if len(errs) != 0 {
		t.Errorf("generated manifest has errors: %v", errs)
	}

	// Verify SKILL.md generation
	skillContent := introspect.GenerateSkillMD(binary, "Go programming language", helpText)
	if !strings.Contains(skillContent, "name: go") {
		t.Error("SKILL.md missing name")
	}
	if !strings.Contains(skillContent, "- go") {
		t.Error("SKILL.md missing trigger")
	}
}

func TestWrapWritesFiles(t *testing.T) {
	dir := t.TempDir()

	m := &manifest.Manifest{
		Name:        "@test/mytool",
		Version:     "0.1.0",
		Type:        manifest.TypeCLI,
		Description: "A test tool",
		CLI:         &manifest.CLISpec{Binary: "mytool", Verify: "mytool --version"},
		Skill:       &manifest.SkillSpec{Entry: "SKILL.md", Origin: "wrapped"},
		Install:     &manifest.InstallSpec{},
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write ctx.yaml: %v", err)
	}

	skillContent := introspect.GenerateSkillMD("mytool", "A test tool", "mytool [options]")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dir, "ctx.yaml")); err != nil {
		t.Error("ctx.yaml not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Error("SKILL.md not created")
	}

	// Verify ctx.yaml is parseable
	parsed, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir error: %v", err)
	}
	if parsed.Name != "@test/mytool" {
		t.Errorf("parsed name = %q, want %q", parsed.Name, "@test/mytool")
	}
	if parsed.Skill == nil || parsed.Skill.Origin != "wrapped" {
		t.Errorf("parsed skill origin = %v, want wrapped", parsed.Skill)
	}
}
