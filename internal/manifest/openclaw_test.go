package manifest

import (
	"strings"
	"testing"
)

func TestParseOpenClawFrontmatter_OpenClaw(t *testing.T) {
	input := `---
name: peekaboo
description: Capture and automate macOS UI
metadata:
  openclaw:
    requires:
      env: [PADEL_AUTH_FILE]
      bins: [padel]
    install:
      brew: [padel]
    config:
      requiredEnv: [PADEL_AUTH_FILE]
      stateDirs: [".config/padel"]
---
# Content
`
	fm, err := ParseOpenClawFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseOpenClawFrontmatter: %v", err)
	}
	if fm == nil {
		t.Fatal("expected non-nil frontmatter")
	}
	if fm.Name != "peekaboo" {
		t.Errorf("Name = %q, want peekaboo", fm.Name)
	}

	oc := fm.GetOpenClawMetadata()
	if oc == nil {
		t.Fatal("expected non-nil OpenClaw metadata")
	}
	if len(oc.Requires.Env) != 1 || oc.Requires.Env[0] != "PADEL_AUTH_FILE" {
		t.Errorf("Requires.Env = %v, want [PADEL_AUTH_FILE]", oc.Requires.Env)
	}
	if len(oc.Requires.Bins) != 1 || oc.Requires.Bins[0] != "padel" {
		t.Errorf("Requires.Bins = %v, want [padel]", oc.Requires.Bins)
	}
	if len(oc.Install.Brew) != 1 || oc.Install.Brew[0] != "padel" {
		t.Errorf("Install.Brew = %v, want [padel]", oc.Install.Brew)
	}
	if len(oc.Config.RequiredEnv) != 1 || oc.Config.RequiredEnv[0] != "PADEL_AUTH_FILE" {
		t.Errorf("Config.RequiredEnv = %v", oc.Config.RequiredEnv)
	}
}

func TestParseOpenClawFrontmatter_ClawdBot(t *testing.T) {
	input := `---
name: test-skill
description: Test
metadata:
  clawdbot:
    requires:
      bins: [ffmpeg]
---
`
	fm, err := ParseOpenClawFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseOpenClawFrontmatter: %v", err)
	}

	oc := fm.GetOpenClawMetadata()
	if oc == nil {
		t.Fatal("expected non-nil metadata for clawdbot alias")
	}
	if len(oc.Requires.Bins) != 1 || oc.Requires.Bins[0] != "ffmpeg" {
		t.Errorf("Requires.Bins = %v, want [ffmpeg]", oc.Requires.Bins)
	}
}

func TestParseOpenClawFrontmatter_NoMetadata(t *testing.T) {
	input := `---
name: simple
description: Simple skill
---
`
	fm, err := ParseOpenClawFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseOpenClawFrontmatter: %v", err)
	}
	if fm == nil {
		t.Fatal("expected non-nil frontmatter")
	}

	oc := fm.GetOpenClawMetadata()
	if oc != nil {
		t.Errorf("expected nil OpenClaw metadata for simple skill")
	}
}

func TestParseOpenClawFrontmatter_NoFrontmatter(t *testing.T) {
	input := "# Just content\nNo frontmatter.\n"

	fm, err := ParseOpenClawFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseOpenClawFrontmatter: %v", err)
	}
	if fm != nil {
		t.Errorf("expected nil frontmatter for content without ---")
	}
}

func TestOpenClawToCtx_RequiresEnv(t *testing.T) {
	fm := &openClawFrontmatter{
		Name:        "padel",
		Description: "Padel tool",
		Metadata: map[string]*OpenClawMetadata{
			"openclaw": {
				Requires: &OpenClawRequires{
					Env:  []string{"PADEL_AUTH_FILE", "API_KEY"},
					Bins: []string{"padel"},
				},
			},
		},
	}

	m := OpenClawToCtx(fm)
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Name != "padel" {
		t.Errorf("Name = %q, want padel", m.Name)
	}
	// Runtime requirements are preserved as keywords (no dedicated field yet).
	envFound := 0
	binFound := 0
	for _, kw := range m.Keywords {
		if kw == "env:PADEL_AUTH_FILE" || kw == "env:API_KEY" {
			envFound++
		}
		if kw == "bin:padel" {
			binFound++
		}
	}
	if envFound != 2 {
		t.Errorf("expected 2 env keywords, got %d: %v", envFound, m.Keywords)
	}
	if binFound != 1 {
		t.Errorf("expected 1 bin keyword, got %d: %v", binFound, m.Keywords)
	}
}

func TestOpenClawToCtx_RequiresBins(t *testing.T) {
	fm := &openClawFrontmatter{
		Name:        "ffmpeg-tool",
		Description: "FFmpeg wrapper",
		Metadata: map[string]*OpenClawMetadata{
			"openclaw": {
				Requires: &OpenClawRequires{
					Bins: []string{"ffmpeg", "ffprobe"},
				},
			},
		},
	}

	m := OpenClawToCtx(fm)
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Runtime requirements are preserved as keywords (no dedicated field yet).
	binCount := 0
	for _, kw := range m.Keywords {
		if kw == "bin:ffmpeg" || kw == "bin:ffprobe" {
			binCount++
		}
	}
	if binCount != 2 {
		t.Errorf("expected 2 bin keywords, got %d: %v", binCount, m.Keywords)
	}
}

func TestOpenClawToCtx_InstallBrew(t *testing.T) {
	fm := &openClawFrontmatter{
		Name:        "ffmpeg-skill",
		Description: "FFmpeg",
		Metadata: map[string]*OpenClawMetadata{
			"openclaw": {
				Install: &OpenClawInstall{
					Brew: []string{"ffmpeg"},
					Node: []string{"@ffmpeg/cli"},
				},
			},
		},
	}

	m := OpenClawToCtx(fm)
	if m.Install == nil {
		t.Fatal("Install is nil")
	}
	if m.Install.Brew != "ffmpeg" {
		t.Errorf("Install.Brew = %q, want ffmpeg", m.Install.Brew)
	}
	if m.Install.Npm != "@ffmpeg/cli" {
		t.Errorf("Install.Npm = %q, want @ffmpeg/cli", m.Install.Npm)
	}
}

func TestOpenClawToCtx_EmptyMetadata(t *testing.T) {
	fm := &openClawFrontmatter{
		Name:        "simple",
		Description: "Simple",
	}

	m := OpenClawToCtx(fm)
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Install != nil {
		t.Errorf("Install should be nil for empty metadata")
	}
}

func TestOpenClawToCtx_NilFrontmatter(t *testing.T) {
	m := OpenClawToCtx(nil)
	if m != nil {
		t.Errorf("expected nil manifest for nil frontmatter")
	}
}

func TestCtxToOpenClaw_Roundtrip(t *testing.T) {
	m := &Manifest{
		Name:        "@test/ffmpeg",
		Version:     "1.0.0",
		Type:        TypeCLI,
		Description: "FFmpeg wrapper",
		Install: &InstallSpec{
			Brew: "ffmpeg",
			Npm:  "@ffmpeg/cli",
		},
		CLI: &CLISpec{
			Binary: "ffmpeg",
			Require: &RequireSpec{
				Bins: []string{"ffmpeg"},
				Env:  []string{"FFMPEG_PATH"},
			},
		},
	}

	oc := CtxToOpenClaw(m)
	if oc == nil {
		t.Fatal("expected non-nil OpenClaw metadata")
	}

	openclaw, ok := oc["openclaw"].(map[string]interface{})
	if !ok {
		t.Fatal("expected openclaw key in metadata")
	}

	install, ok := openclaw["install"].(map[string]interface{})
	if !ok {
		t.Fatal("expected install in openclaw")
	}
	brew := install["brew"].([]string)
	if len(brew) != 1 || brew[0] != "ffmpeg" {
		t.Errorf("brew = %v, want [ffmpeg]", brew)
	}

	requires, ok := openclaw["requires"].(map[string]interface{})
	if !ok {
		t.Fatal("expected requires in openclaw")
	}
	bins := requires["bins"].([]string)
	if len(bins) != 1 || bins[0] != "ffmpeg" {
		t.Errorf("bins = %v, want [ffmpeg]", bins)
	}
}

func TestCtxToOpenClaw_NoInstallNoRequire(t *testing.T) {
	m := &Manifest{
		Name:        "@test/simple",
		Version:     "1.0.0",
		Type:        TypeSkill,
		Description: "Simple skill",
		Skill:       &SkillSpec{Entry: "SKILL.md"},
	}

	oc := CtxToOpenClaw(m)
	if oc != nil {
		t.Errorf("expected nil OpenClaw metadata for simple skill")
	}
}
