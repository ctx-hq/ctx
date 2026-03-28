package manifest

import (
	"strings"
	"testing"
)

func TestParseSkillMD_Full(t *testing.T) {
	content := `---
name: basecamp
description: |
  Interact with Basecamp via CLI.
triggers:
  - basecamp
  - /basecamp
  - basecamp todo
invocable: true
argument-hint: "[action] [args...]"
---

# /basecamp - Basecamp Command

## Quick Reference
Some content here.
`

	fm, body, err := ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if fm == nil {
		t.Fatal("expected frontmatter, got nil")
	}
	if fm.Name != "basecamp" {
		t.Errorf("name = %q, want %q", fm.Name, "basecamp")
	}
	if len(fm.Triggers) != 3 {
		t.Errorf("triggers len = %d, want 3", len(fm.Triggers))
	}
	if !fm.Invocable {
		t.Error("invocable should be true")
	}
	if fm.ArgumentHint != "[action] [args...]" {
		t.Errorf("argument-hint = %q", fm.ArgumentHint)
	}
	if !strings.Contains(body, "Quick Reference") {
		t.Error("body should contain Quick Reference section")
	}
}

func TestParseSkillMD_NoFrontmatter(t *testing.T) {
	content := `# Just a plain markdown file

No frontmatter here.
`

	fm, body, err := ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if fm != nil {
		t.Error("expected nil frontmatter for plain markdown")
	}
	if !strings.Contains(body, "Just a plain markdown file") {
		t.Error("body should contain full content")
	}
}

func TestParseSkillMD_EmptyFile(t *testing.T) {
	fm, body, err := ParseSkillMD(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if fm != nil {
		t.Error("expected nil frontmatter for empty file")
	}
	if body != "" {
		t.Errorf("body should be empty, got %q", body)
	}
}

func TestParseSkillMD_MalformedYAML(t *testing.T) {
	content := `---
name: [invalid yaml
---
Body content.
`

	_, _, err := ParseSkillMD(strings.NewReader(content))
	if err == nil {
		t.Error("expected error for malformed YAML frontmatter")
	}
}

func TestParseSkillMD_MinimalFrontmatter(t *testing.T) {
	content := `---
name: test
invocable: false
---
`

	fm, _, err := ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if fm.Name != "test" {
		t.Errorf("name = %q, want %q", fm.Name, "test")
	}
	if fm.Invocable {
		t.Error("invocable should be false")
	}
}

func TestValidateSkillMD_Full(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:         "review",
		Description:  "Code review skill",
		Triggers:     []string{"review", "/review", "code review"},
		Invocable:    true,
		ArgumentHint: "[action]",
	}
	m := &Manifest{Name: "@hong/review"}

	warnings := ValidateSkillMD(fm, m)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for valid skill, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateSkillMD_NameMismatch(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:        "basecamp",
		Description: "test",
		Triggers:    []string{"a", "b", "c"},
	}
	m := &Manifest{Name: "@hong/review"}

	warnings := ValidateSkillMD(fm, m)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "differs") {
			found = true
		}
	}
	if !found {
		t.Error("expected name mismatch warning")
	}
}

func TestValidateSkillMD_MissingFields(t *testing.T) {
	fm := &SkillFrontmatter{
		Invocable: true,
		// Missing: name, description, triggers, argument-hint
	}

	warnings := ValidateSkillMD(fm, nil)
	if len(warnings) < 3 {
		t.Errorf("expected at least 3 warnings for missing fields, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateSkillMD_NilFrontmatter(t *testing.T) {
	warnings := ValidateSkillMD(nil, nil)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for nil frontmatter, got %d", len(warnings))
	}
}

func TestValidateSkillMD_FewTriggers(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:        "test",
		Description: "test",
		Triggers:    []string{"one"},
	}

	warnings := ValidateSkillMD(fm, nil)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "few triggers") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about few triggers")
	}
}
