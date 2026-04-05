package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/importer"
	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestIsSingleFile(t *testing.T) {
	dir := t.TempDir()

	// Create a .md file
	mdFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(mdFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a .txt file
	txtFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"md file", mdFile, true},
		{"txt file", txtFile, false},
		{"directory", dir, false},
		{"nonexistent", filepath.Join(dir, "nope.md"), false},
		{"uppercase md no file", filepath.Join(dir, "nope.MD"), false}, // file doesn't exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSingleFile(tt.path)
			if got != tt.want {
				t.Errorf("isSingleFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gc", "gc"},
		{"my-skill", "my-skill"},
		{"My Skill", "my-skill"},
		{"UPPERCASE", "uppercase"},
		{"with spaces and stuff!", "with-spaces-and-stuff"},
		{"  leading-trailing  ", "leading-trailing"},
		{"multi---hyphens", "multi-hyphens"},
		{"CamelCase", "camelcase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := importer.Slugify(tt.input)
			if got != tt.want {
				t.Errorf("importer.Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"first content line",
			"# Title\n\nThis is the description.\n\nMore content.",
			"This is the description.",
		},
		{
			"skips headings",
			"# Heading\n## Subheading\nActual content here.",
			"Actual content here.",
		},
		{
			"empty body",
			"",
			"",
		},
		{
			"only headings",
			"# Heading\n## Sub\n### Sub2\n",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDescription(tt.body)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"请根据当前代码改动生成提交信息", 10, "请根据当前代码..."},
		{"你好世界hello", 6, "你好世..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestBuildManifestFromFile(t *testing.T) {
	// Test the metadata merging logic
	fm := &manifest.SkillFrontmatter{
		Name:        "gc",
		Description: "Generate bilingual commit messages",
		Triggers:    []string{"/gc", "git commit", "提交信息"},
		Invocable:   true,
	}

	scope := "biao29"
	m := manifest.Scaffold(manifest.TypeSkill, scope, fm.Name)
	m.Version = "0.1.0"
	m.Description = fm.Description
	m.Visibility = "private"
	invocable := true
	m.Skill.UserInvocable = &invocable

	// Validate
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Errorf("validation errors: %v", errs)
	}

	if m.Name != "@biao29/gc" {
		t.Errorf("name = %q, want %q", m.Name, "@biao29/gc")
	}
	if m.Type != manifest.TypeSkill {
		t.Errorf("type = %q, want %q", m.Type, manifest.TypeSkill)
	}
	if m.Skill.Entry != "SKILL.md" {
		t.Errorf("entry = %q, want %q", m.Skill.Entry, "SKILL.md")
	}
}

func TestStagingIntegration(t *testing.T) {
	// Simulate the staging flow without network calls
	fm := &manifest.SkillFrontmatter{
		Name:        "test-skill",
		Description: "A test",
		Triggers:    []string{"/test"},
		Invocable:   true,
	}
	body := "# Test\n\nContent here.\n"

	m := manifest.Scaffold(manifest.TypeSkill, "testuser", "test-skill")
	m.Version = "0.1.0"
	m.Description = fm.Description
	m.Visibility = "private"

	// Marshal
	manifestData, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	skillContent, err := manifest.RenderSkillMD(fm, body)
	if err != nil {
		t.Fatal(err)
	}

	// Write to dest dir directly (simulating staging commit)
	dest := filepath.Join(t.TempDir(), "testuser", "test-skill")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "ctx.yaml"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), skillContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify files exist and are valid
	loaded, err := manifest.LoadFromDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "@testuser/test-skill" {
		t.Errorf("loaded name = %q", loaded.Name)
	}

	// Verify SKILL.md roundtrips
	f, err := os.Open(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	parsedFM, parsedBody, err := manifest.ParseSkillMD(f)
	if err != nil {
		t.Fatal(err)
	}
	if parsedFM == nil {
		t.Fatal("parsed frontmatter is nil")
	}
	if parsedFM.Name != "test-skill" {
		t.Errorf("parsed name = %q", parsedFM.Name)
	}
	if !strings.Contains(parsedBody, "Content here.") {
		t.Errorf("parsed body missing content: %q", parsedBody)
	}
}

func TestResolveBaseVersion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// No existing package → default 0.1.0
	got := resolveBaseVersion("testuser", "nonexistent")
	if got != "0.1.0" {
		t.Errorf("no existing: got %q, want %q", got, "0.1.0")
	}

	// Create an existing package with version 1.2.3
	pkgDir := filepath.Join(tmp, "skills", "testuser", "my-skill")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.Scaffold(manifest.TypeSkill, "testuser", "my-skill")
	m.Version = "1.2.3"
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got = resolveBaseVersion("testuser", "my-skill")
	if got != "1.2.3" {
		t.Errorf("existing: got %q, want %q", got, "1.2.3")
	}
}

func TestLinkToOriginal_Atomic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	original := filepath.Join(tmp, "gc.md")
	target := filepath.Join(tmp, "skills", "SKILL.md")

	// Create original file
	if err := os.WriteFile(original, []byte("original content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create target
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("skill content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Link
	if err := linkToOriginal(original, target, "@test/gc"); err != nil {
		t.Fatal(err)
	}

	// Original should be a symlink
	link, err := os.Readlink(original)
	if err != nil {
		t.Fatalf("original should be a symlink: %v", err)
	}
	if link != target {
		t.Errorf("symlink target = %q, want %q", link, target)
	}

	// Backup should exist
	bakData, err := os.ReadFile(original + ".bak")
	if err != nil {
		t.Fatalf("backup should exist: %v", err)
	}
	if string(bakData) != "original content" {
		t.Errorf("backup content = %q", string(bakData))
	}

	// Calling again should be idempotent (already linked)
	if err := linkToOriginal(original, target, "@test/gc"); err != nil {
		t.Fatal(err)
	}
}
