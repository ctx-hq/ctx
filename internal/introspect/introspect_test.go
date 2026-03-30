package introspect

import (
	"strings"
	"testing"
)

func TestCaptureHelp(t *testing.T) {
	// "go" is reliably available in test environments
	help, err := CaptureHelp("go")
	if err != nil {
		t.Fatalf("CaptureHelp(go) error: %v", err)
	}
	if !strings.Contains(help, "Go is a tool") {
		t.Errorf("CaptureHelp(go) output doesn't contain expected text, got: %s", help[:min(len(help), 200)])
	}
}

func TestCaptureHelp_NotFound(t *testing.T) {
	_, err := CaptureHelp("nonexistent-binary-xyz")
	if err == nil {
		t.Error("CaptureHelp(nonexistent) should return error")
	}
}

func TestCaptureVersion(t *testing.T) {
	vr := CaptureVersion("go")
	if vr.Version == "0.1.0" {
		t.Skip("go version not parseable as semver, skipping")
	}
	// Should extract something like "1.22.0"
	if !strings.Contains(vr.Version, ".") {
		t.Errorf("CaptureVersion(go).Version = %q, expected semver-like", vr.Version)
	}
	if vr.Command != "--version" && vr.Command != "version" {
		t.Errorf("CaptureVersion(go).Command = %q, expected --version or version", vr.Command)
	}
}

func TestCaptureVersion_NotFound(t *testing.T) {
	vr := CaptureVersion("nonexistent-binary-xyz")
	if vr.Version != "0.1.0" {
		t.Errorf("CaptureVersion(nonexistent).Version = %q, want %q", vr.Version, "0.1.0")
	}
	if vr.Command != "--version" {
		t.Errorf("CaptureVersion(nonexistent).Command = %q, want %q", vr.Command, "--version")
	}
}

func TestDetectInstallMethod(t *testing.T) {
	// Just verify it doesn't panic and returns a non-nil spec
	spec := DetectInstallMethod("go")
	if spec == nil {
		t.Fatal("DetectInstallMethod returned nil")
	}
}

func TestGenerateSkillMD(t *testing.T) {
	md := GenerateSkillMD("ffmpeg", "Multimedia processing toolkit", "ffmpeg [options] input output")

	if !strings.Contains(md, "name: ffmpeg") {
		t.Error("missing name in frontmatter")
	}
	if !strings.Contains(md, "- ffmpeg") {
		t.Error("missing trigger")
	}
	if !strings.Contains(md, "- /ffmpeg") {
		t.Error("missing slash trigger")
	}
	if !strings.Contains(md, "# ffmpeg") {
		t.Error("missing heading")
	}
	if !strings.Contains(md, "```") {
		t.Error("missing code block for help text")
	}
	if !strings.Contains(md, "ffmpeg [options] input output") {
		t.Error("missing help text content")
	}
}

func TestGenerateSkillMD_NoHelp(t *testing.T) {
	md := GenerateSkillMD("mytool", "A tool", "")
	if strings.Contains(md, "```") {
		t.Error("should not have code block when no help text")
	}
	if !strings.Contains(md, "# mytool") {
		t.Error("missing heading")
	}
}

func TestGenerateSkillMD_LongHelp(t *testing.T) {
	longHelp := strings.Repeat("x", 5000)
	md := GenerateSkillMD("tool", "desc", longHelp)
	if !strings.Contains(md, "... (truncated)") {
		t.Error("long help should be truncated")
	}
}

