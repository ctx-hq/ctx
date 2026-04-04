package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestEnrichFromSkillMD_FillsDescription(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\ndescription: A test skill for testing\n---\n# Content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:  manifest.TypeSkill,
		Skill: &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil)

	if m.Description != "A test skill for testing" {
		t.Errorf("description = %q, want %q", m.Description, "A test skill for testing")
	}
}

func TestEnrichFromSkillMD_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\ndescription: From SKILL.md\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:        manifest.TypeSkill,
		Description: "From ctx.yaml",
		Skill:       &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil)

	if m.Description != "From ctx.yaml" {
		t.Errorf("description = %q, want %q (should not overwrite)", m.Description, "From ctx.yaml")
	}
}

func TestEnrichFromSkillMD_NoSkillEntry(t *testing.T) {
	dir := t.TempDir()
	m := &manifest.Manifest{
		Type: manifest.TypeMCP,
	}
	enrichFromSkillMD(m, dir, nil) // should be a no-op, not panic
	if m.Description != "" {
		t.Errorf("description = %q, want empty", m.Description)
	}
}

func TestEnrichFromSkillMD_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Just a markdown file\nNo frontmatter here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:  manifest.TypeSkill,
		Skill: &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil)

	if m.Description != "" {
		t.Errorf("description = %q, want empty (no frontmatter)", m.Description)
	}
}

func TestEnrichFromSkillMD_TriggersNotCopiedToKeywords(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: test\ndescription: Test\ntriggers:\n  - /gc\n  - git commit\n---\n# Content\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:  manifest.TypeSkill,
		Skill: &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil)

	if len(m.Keywords) != 0 {
		t.Errorf("keywords = %v, want empty (triggers should not become keywords)", m.Keywords)
	}
}

func TestEnrichFromSkillMD_TruncatesLongDescription(t *testing.T) {
	dir := t.TempDir()
	longDesc := make([]byte, 2000)
	for i := range longDesc {
		longDesc[i] = 'a'
	}
	content := "---\nname: test\ndescription: " + string(longDesc) + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:  manifest.TypeSkill,
		Skill: &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil)

	if len(m.Description) > 1024 {
		t.Errorf("description length = %d, want <= 1024", len(m.Description))
	}
	if m.Description[len(m.Description)-3:] != "..." {
		t.Errorf("truncated description should end with '...', got %q", m.Description[len(m.Description)-5:])
	}
}

func TestEnrichFromSkillMD_MissingFile(t *testing.T) {
	dir := t.TempDir()
	m := &manifest.Manifest{
		Type:  manifest.TypeSkill,
		Skill: &manifest.SkillSpec{Entry: "SKILL.md"},
	}
	enrichFromSkillMD(m, dir, nil) // file doesn't exist, should not panic
	if m.Description != "" {
		t.Errorf("description = %q, want empty", m.Description)
	}
}

func TestAppendPrerelease(t *testing.T) {
	tests := []struct {
		version    string
		tag        string
		wantPrefix string // expected prefix before the 14-digit timestamp
	}{
		{"1.2.0", "canary", "1.2.0-canary."},
		{"0.1.0", "beta", "0.1.0-beta."},
		{"2.0.0", "rc", "2.0.0-rc."},
		{"1.0.0-alpha.1", "canary", "1.0.0-canary."},   // strips existing prerelease
		{"3.0.0+build123", "beta", "3.0.0-beta."},       // strips existing metadata
		{"1.0.0-rc.1+meta", "canary", "1.0.0-canary."}, // strips both
	}
	for _, tt := range tests {
		result := appendPrerelease(tt.version, tt.tag)
		if !strings.HasPrefix(result, tt.wantPrefix) {
			t.Errorf("appendPrerelease(%q, %q) = %q, want prefix %q", tt.version, tt.tag, result, tt.wantPrefix)
		}
		// Timestamp should be 14 digits
		ts := strings.TrimPrefix(result, tt.wantPrefix)
		if len(ts) != 14 {
			t.Errorf("timestamp part = %q, want 14 digits", ts)
		}
	}
}

func TestEnrichFromSkillMD_KeywordsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: test\ndescription: Test\ntriggers:\n  - /gc\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Type:     manifest.TypeSkill,
		Skill:    &manifest.SkillSpec{Entry: "SKILL.md"},
		Keywords: []string{"existing"},
	}
	enrichFromSkillMD(m, dir, nil)

	if len(m.Keywords) != 1 || m.Keywords[0] != "existing" {
		t.Errorf("keywords = %v, want [existing] (should not overwrite)", m.Keywords)
	}
}
