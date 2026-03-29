package manifest

import (
	"strings"
	"testing"
)

func TestRenderSkillMD_NilFrontmatter(t *testing.T) {
	body := "# My Skill\n\nSome content.\n"
	got, err := RenderSkillMD(nil, body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("got %q, want %q", string(got), body)
	}
}

func TestRenderSkillMD_WithFrontmatter(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:        "test-skill",
		Description: "A test skill",
		Triggers:    []string{"/test", "test skill"},
		Invocable:   true,
	}
	body := "# Test Skill\n\nContent here.\n"

	got, err := RenderSkillMD(fm, body)
	if err != nil {
		t.Fatal(err)
	}

	result := string(got)

	// Must start with ---
	if !strings.HasPrefix(result, "---\n") {
		t.Error("must start with ---")
	}

	// Must contain closing ---
	parts := strings.SplitN(result, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("expected frontmatter delimiters, got: %s", result)
	}

	// Body must be present
	if !strings.Contains(result, "# Test Skill") {
		t.Error("body not found in output")
	}

	// Frontmatter must contain key fields
	fmSection := parts[1]
	if !strings.Contains(fmSection, "name: test-skill") {
		t.Errorf("frontmatter missing name, got: %s", fmSection)
	}
	if !strings.Contains(fmSection, "description: A test skill") {
		t.Errorf("frontmatter missing description, got: %s", fmSection)
	}
	if !strings.Contains(fmSection, "invocable: true") {
		t.Errorf("frontmatter missing invocable, got: %s", fmSection)
	}
}

func TestRenderSkillMD_Roundtrip(t *testing.T) {
	original := &SkillFrontmatter{
		Name:         "gc",
		Description:  "Generate bilingual commit messages",
		Triggers:     []string{"/gc", "git commit", "commit message"},
		Invocable:    true,
		ArgumentHint: "",
	}
	body := "# GC Skill\n\nGenerates commit messages.\n"

	rendered, err := RenderSkillMD(original, body)
	if err != nil {
		t.Fatal(err)
	}

	// Parse it back
	parsed, parsedBody, err := ParseSkillMD(strings.NewReader(string(rendered)))
	if err != nil {
		t.Fatal(err)
	}

	if parsed == nil {
		t.Fatal("parsed frontmatter is nil")
	}

	if parsed.Name != original.Name {
		t.Errorf("name: got %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Description != original.Description {
		t.Errorf("description: got %q, want %q", parsed.Description, original.Description)
	}
	if len(parsed.Triggers) != len(original.Triggers) {
		t.Errorf("triggers count: got %d, want %d", len(parsed.Triggers), len(original.Triggers))
	}
	if parsed.Invocable != original.Invocable {
		t.Errorf("invocable: got %v, want %v", parsed.Invocable, original.Invocable)
	}

	// Body should match (may have leading newline difference)
	trimmedBody := strings.TrimLeft(parsedBody, "\n")
	trimmedOriginal := strings.TrimLeft(body, "\n")
	if trimmedBody != trimmedOriginal {
		t.Errorf("body mismatch:\ngot:  %q\nwant: %q", trimmedBody, trimmedOriginal)
	}
}

func TestRenderSkillMD_EmptyBody(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:        "empty",
		Description: "Empty skill",
	}

	got, err := RenderSkillMD(fm, "")
	if err != nil {
		t.Fatal(err)
	}

	result := string(got)
	if !strings.HasPrefix(result, "---\n") {
		t.Error("must start with ---")
	}
	if !strings.Contains(result, "name: empty") {
		t.Error("missing name in frontmatter")
	}
}

func TestRenderSkillMD_SpecialCharsInDescription(t *testing.T) {
	fm := &SkillFrontmatter{
		Name:        "special",
		Description: `A skill with "quotes" and colons: here`,
	}

	rendered, err := RenderSkillMD(fm, "body\n")
	if err != nil {
		t.Fatal(err)
	}

	// Roundtrip should preserve
	parsed, _, err := ParseSkillMD(strings.NewReader(string(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Description != fm.Description {
		t.Errorf("description mismatch: got %q, want %q", parsed.Description, fm.Description)
	}
}
