package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
)

// helper to create files in a temp directory.
func writeFixture(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectImportFormat(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string)
		wantFmt importFormat
		wantN   int // expected number of detected skills
	}{
		{
			name: "marketplace_json",
			setup: func(dir string) {
				writeFixture(t, dir, ".claude-plugin/marketplace.json", `{
					"name": "test", "plugins": [{
						"name": "p1", "skills": ["./skills/a", "./skills/b"]
					}]
				}`)
				writeFixture(t, dir, "skills/a/SKILL.md", "---\nname: a\ndescription: Skill A\n---\n")
				writeFixture(t, dir, "skills/b/SKILL.md", "---\nname: b\ndescription: Skill B\n---\n")
			},
			wantFmt: importFormatMarketplace,
			wantN:   2,
		},
		{
			name: "codex_curated",
			setup: func(dir string) {
				writeFixture(t, dir, "skills/.curated/gc/SKILL.md", "---\nname: gc\ndescription: Git commit\n---\n")
				writeFixture(t, dir, "skills/.curated/review/SKILL.md", "---\nname: review\ndescription: Review\n---\n")
			},
			wantFmt: importFormatCodex,
			wantN:   2,
		},
		{
			name: "codex_system",
			setup: func(dir string) {
				writeFixture(t, dir, "skills/.system/creator/SKILL.md", "---\nname: creator\n---\n")
			},
			wantFmt: importFormatCodex,
			wantN:   1,
		},
		{
			name: "single_skill_with_frontmatter",
			setup: func(dir string) {
				writeFixture(t, dir, "SKILL.md", "---\nname: my-skill\ndescription: A skill\n---\n# Content\n")
			},
			wantFmt: importFormatSingleSkill,
			wantN:   1,
		},
		{
			name: "flat_skill_dirs",
			setup: func(dir string) {
				writeFixture(t, dir, "gc/SKILL.md", "---\nname: gc\n---\n")
				writeFixture(t, dir, "review/SKILL.md", "---\nname: review\n---\n")
			},
			wantFmt: importFormatFlatSkills,
			wantN:   2,
		},
		{
			name: "nested_skill_dirs",
			setup: func(dir string) {
				writeFixture(t, dir, "engineering/gc/SKILL.md", "---\nname: gc\n---\n")
				writeFixture(t, dir, "writing/translate/SKILL.md", "---\nname: translate\n---\n")
			},
			wantFmt: importFormatNestedSkills,
			wantN:   2,
		},
		{
			name: "bare_markdown",
			setup: func(dir string) {
				writeFixture(t, dir, "minutes.md", "# Minutes\nNo frontmatter here.\n")
			},
			wantFmt: importFormatBareMarkdown,
			wantN:   1,
		},
		{
			name: "unknown_empty_repo",
			setup: func(dir string) {
				writeFixture(t, dir, "README.md", "# Just a readme\n")
			},
			wantFmt: importFormatUnknown,
			wantN:   0,
		},
		{
			name: "readme_only_not_bare",
			setup: func(dir string) {
				// README.md alone should NOT be detected as bare markdown
				writeFixture(t, dir, "README.md", "# Project\nSome content\n")
				writeFixture(t, dir, "CHANGELOG.md", "# Changelog\n")
			},
			wantFmt: importFormatUnknown,
			wantN:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			det, err := detectImportFormat(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if det.format != tt.wantFmt {
				t.Errorf("format = %v, want %v", det.format, tt.wantFmt)
			}
			if len(det.skills) != tt.wantN {
				t.Errorf("skills count = %d, want %d", len(det.skills), tt.wantN)
			}
		})
	}
}

func TestDetectImportFormat_MarketplaceNonexistentDir(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude-plugin/marketplace.json", `{
		"name": "test", "plugins": [{
			"name": "p1", "skills": ["./skills/exists", "./skills/missing"]
		}]
	}`)
	writeFixture(t, dir, "skills/exists/SKILL.md", "---\nname: exists\n---\n")
	// skills/missing does NOT exist

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if det.format != importFormatMarketplace {
		t.Errorf("format = %v, want marketplace", det.format)
	}
	if len(det.skills) != 1 {
		t.Errorf("skills = %d, want 1 (missing dir skipped)", len(det.skills))
	}
}

func TestDetectImportFormat_SkillMDNoName(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "SKILL.md", "---\ndescription: A skill without name\n---\n# Content\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if det.format != importFormatSingleSkill {
		t.Errorf("format = %v, want single-skill", det.format)
	}
	if len(det.skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(det.skills))
	}
	// Name should fall back to directory basename
	if det.skills[0].name == "" {
		t.Error("skill name should not be empty (should fall back to dir basename)")
	}
}

func TestDetectImportFormat_SkipInternalDir(t *testing.T) {
	// Simulates fastmail-cli: internal/skills/SKILL.md + skills/fastmail/SKILL.md
	// internal/ should be excluded, and the duplicate name should be deduped → single skill
	dir := t.TempDir()
	writeFixture(t, dir, "internal/skills/SKILL.md", "---\nname: fastmail\ndescription: CLI tool\n---\n")
	writeFixture(t, dir, "skills/fastmail/SKILL.md", "---\nname: fastmail\ndescription: CLI tool\n---\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if det.format != importFormatSingleSkill {
		t.Errorf("format = %v, want single-skill (internal/ excluded + dedup)", det.format)
	}
	if len(det.skills) != 1 {
		t.Errorf("skills = %d, want 1 (deduped)", len(det.skills))
	}
	if len(det.skills) > 0 && det.skills[0].name != "fastmail" {
		t.Errorf("skill name = %q, want fastmail", det.skills[0].name)
	}
}

func TestDetectImportFormat_DedupSameName(t *testing.T) {
	// Two directories with the same skill name → deduplicated to 1
	dir := t.TempDir()
	writeFixture(t, dir, "a/SKILL.md", "---\nname: my-skill\n---\n")
	writeFixture(t, dir, "b/SKILL.md", "---\nname: my-skill\n---\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Two dirs with same name → dedup to 1 → single-skill
	if det.format != importFormatSingleSkill {
		t.Errorf("format = %v, want single-skill (deduped)", det.format)
	}
	if len(det.skills) != 1 {
		t.Errorf("skills = %d, want 1", len(det.skills))
	}
}

func TestContainsExcludedDir(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"skills/translate", false},
		{"internal/skills", true},
		{"cmd/main", true},
		{"vendor/pkg", true},
		{"node_modules/foo", true},
		{".github/workflows", true},
		{"engineering/gc", false},
		{"pkg/util", true},
	}
	for _, tt := range tests {
		got := containsExcludedDir(tt.path)
		if got != tt.want {
			t.Errorf("containsExcludedDir(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDetectImportFormat_FlatSkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "gc/SKILL.md", "---\nname: gc\n---\n")
	writeFixture(t, dir, ".hidden/SKILL.md", "---\nname: hidden\n---\n")
	writeFixture(t, dir, "node_modules/SKILL.md", "---\nname: nm\n---\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(det.skills) != 1 {
		t.Errorf("skills = %d, want 1 (hidden and node_modules skipped)", len(det.skills))
	}
}

func TestBuildSkillManifest(t *testing.T) {
	skill := importedSkill{
		name:        "translate",
		description: "Translate text",
		version:     "1.2.0",
		tags:        []string{"i18n", "translate"},
	}
	m := buildManifest(skill, "baoyu", t.TempDir())
	if m.Name != "@baoyu/translate" {
		t.Errorf("name = %q, want @baoyu/translate", m.Name)
	}
	if m.Version != "1.2.0" {
		t.Errorf("version = %q, want 1.2.0", m.Version)
	}
	if m.Description != "Translate text" {
		t.Errorf("description = %q", m.Description)
	}
	if len(m.Keywords) != 2 {
		t.Errorf("keywords = %v, want 2", m.Keywords)
	}
}

func TestBuildManifest_Defaults(t *testing.T) {
	skill := importedSkill{name: "test"}
	m := buildManifest(skill, "", t.TempDir())
	if m.Name != "test" {
		t.Errorf("name = %q, want 'test' (no scope)", m.Name)
	}
	if m.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", m.Version)
	}
}

func TestBuildManifest_CLIDetection(t *testing.T) {
	// Simulate a Go CLI project
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module github.com/example/cli\n\ngo 1.24\n")
	os.MkdirAll(filepath.Join(dir, "cmd", "myapp"), 0o755)

	skill := importedSkill{name: "myapp", description: "A CLI tool"}
	m := buildManifest(skill, "user", dir)

	if m.Type != manifest.TypeCLI {
		t.Errorf("type = %q, want cli (go.mod + cmd/ detected)", m.Type)
	}
	if m.CLI == nil {
		t.Fatal("cli section is nil")
	}
	if m.CLI.Binary != "myapp" {
		t.Errorf("cli.binary = %q, want myapp", m.CLI.Binary)
	}
}

func TestBuildManifest_SkillEntryPath(t *testing.T) {
	// When skill is in a subdirectory and ctx.yaml is at root,
	// entry should include the relative path
	skill := importedSkill{
		name:  "fastmail",
		dir:   "skills/fastmail",
		entry: "SKILL.md",
	}
	m := buildManifest(skill, "user", t.TempDir())
	want := filepath.Join("skills/fastmail", "SKILL.md")
	if m.Skill.Entry != want {
		t.Errorf("skill.entry = %q, want %q", m.Skill.Entry, want)
	}
}

func TestBuildManifest_KeywordsCapped(t *testing.T) {
	tags := make([]string, 30)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag-%d", i)
	}
	skill := importedSkill{name: "test", tags: tags}
	m := buildManifest(skill, "", t.TempDir())
	if len(m.Keywords) > 10 {
		t.Errorf("keywords = %d, want <= 10 (should be capped)", len(m.Keywords))
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  manifest.PackageType
	}{
		{
			name: "go_cli",
			setup: func(dir string) {
				writeFixture(t, dir, "go.mod", "module test\n")
				os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
			},
			want: manifest.TypeCLI,
		},
		{
			name: "goreleaser",
			setup: func(dir string) {
				writeFixture(t, dir, ".goreleaser.yaml", "builds:\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "plain_skill",
			setup: func(dir string) {
				writeFixture(t, dir, "SKILL.md", "---\nname: test\n---\n")
			},
			want: manifest.TypeSkill,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := detectProjectType(dir)
			if got != tt.want {
				t.Errorf("detectProjectType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMemberGlobs(t *testing.T) {
	tests := []struct {
		name   string
		skills []importedSkill
		want   []string
	}{
		{
			name:   "common_prefix",
			skills: []importedSkill{{dir: "skills/a"}, {dir: "skills/b"}, {dir: "skills/c"}},
			want:   []string{"skills/*"},
		},
		{
			name:   "root_level",
			skills: []importedSkill{{dir: "a"}, {dir: "b"}},
			want:   []string{"*"},
		},
		{
			name:   "two_levels",
			skills: []importedSkill{{dir: "eng/gc/sub"}, {dir: "eng/review/sub"}},
			want:   []string{"eng/*/*"},
		},
		{
			name:   "empty",
			skills: nil,
			want:   []string{"*"},
		},
		{
			name:   "multi_prefix",
			skills: []importedSkill{{dir: "engineering/gc"}, {dir: "writing/translate"}},
			want:   []string{"engineering/*", "writing/*"},
		},
		{
			name:   "multi_prefix_with_counts",
			skills: []importedSkill{{dir: "eng/gc"}, {dir: "eng/review"}, {dir: "writing/translate"}},
			want:   []string{"eng/*", "writing/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferMemberGlobs(tt.skills)
			if len(got) != len(tt.want) {
				t.Fatalf("inferMemberGlobs = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("inferMemberGlobs[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWriteReleasePleaseConfig(t *testing.T) {
	dir := t.TempDir()
	det := &importDetection{
		skills: []importedSkill{
			{dir: "skills/a", version: "1.0.0"},
			{dir: "skills/b", version: "2.1.0"},
		},
	}
	w := output.NewWriter()

	n, err := writeReleasePleaseConfig(dir, det, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("files written = %d, want 2", n)
	}

	// Verify config
	configData, err := os.ReadFile(filepath.Join(dir, "release-please-config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	packages, ok := config["packages"].(map[string]interface{})
	if !ok {
		t.Fatal("packages not found in config")
	}
	if _, ok := packages["skills/a"]; !ok {
		t.Error("skills/a not in config packages")
	}
	if _, ok := packages["skills/b"]; !ok {
		t.Error("skills/b not in config packages")
	}

	// Verify manifest
	manifestData, err := os.ReadFile(filepath.Join(dir, ".release-please-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var versions map[string]string
	if err := json.Unmarshal(manifestData, &versions); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if versions["skills/a"] != "1.0.0" {
		t.Errorf("skills/a version = %q, want 1.0.0", versions["skills/a"])
	}
	if versions["skills/b"] != "2.1.0" {
		t.Errorf("skills/b version = %q, want 2.1.0", versions["skills/b"])
	}
}

func TestWriteReleasePleaseConfig_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "release-please-config.json", `{"existing": true}`)
	writeFixture(t, dir, ".release-please-manifest.json", `{"existing": "1.0.0"}`)

	det := &importDetection{
		skills: []importedSkill{{dir: "skills/a"}},
	}
	w := output.NewWriter()

	n, err := writeReleasePleaseConfig(dir, det, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("files written = %d, want 0 (should skip existing)", n)
	}
}
