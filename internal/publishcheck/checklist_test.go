package publishcheck

import (
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestFormatChecklistCLI(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@test/fizzy-cli",
		Version:     "0.1.2",
		Type:        manifest.TypeCLI,
		Description: "CLI for Fizzy",
		CLI: &manifest.CLISpec{
			Binary: "fizzy",
			Verify: "fizzy --version",
			Auth:   "Run 'fizzy setup' to configure your API token",
		},
		Skill: &manifest.SkillSpec{
			Entry: "skills/fizzy/SKILL.md",
		},
	}

	results := []CheckResult{
		{Method: "script", Pkg: "https://example.com/install.sh", OK: true},
	}

	out := FormatChecklist(m, results)

	if !strings.Contains(out, "@test/fizzy-cli@0.1.2") {
		t.Error("should contain package name and version")
	}
	if !strings.Contains(out, "fizzy") {
		t.Error("should contain binary name")
	}
	if !strings.Contains(out, "[x]") {
		t.Error("should contain check marks")
	}
	if !strings.Contains(out, "Auth hint") {
		t.Error("should contain auth hint")
	}
	if !strings.Contains(out, "script") {
		t.Error("should contain install method")
	}
}

func TestFormatChecklistWithIssues(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@test/broken",
		Version:     "1.0.0",
		Type:        manifest.TypeCLI,
		Description: "Test",
		CLI: &manifest.CLISpec{
			Binary: "broken",
		},
	}

	results := []CheckResult{
		{Method: "brew", Pkg: "nonexistent/tap/broken", OK: false, Error: "formula not found"},
	}

	out := FormatChecklist(m, results)

	if !strings.Contains(out, "[!]") {
		t.Error("should contain failure markers")
	}
	if !strings.Contains(out, "formula not found") {
		t.Error("should contain error message")
	}
}

func TestFormatChecklistSkill(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@test/my-skill",
		Version:     "1.0.0",
		Type:        manifest.TypeSkill,
		Description: "A test skill",
	}

	out := FormatChecklist(m, nil)

	if !strings.Contains(out, "skill") {
		t.Error("should contain type")
	}
	// Skills don't have install methods
	if strings.Contains(out, "Install") {
		t.Error("skills should not show install line")
	}
}
