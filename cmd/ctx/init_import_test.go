package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	m := buildManifest(skill, "baoyu", t.TempDir(), "", "")
	if m.Name != "@baoyu/translate" {
		t.Errorf("name = %q, want @baoyu/translate", m.Name)
	}
	if m.Version != "1.2.0" {
		t.Errorf("version = %q, want 1.2.0", m.Version)
	}
	if m.Description != "Translate text" {
		t.Errorf("description = %q", m.Description)
	}
	// Triggers go to both skill.tags and keywords
	if len(m.Keywords) != 2 {
		t.Errorf("keywords = %v, want 2", m.Keywords)
	}
	if m.Skill == nil || len(m.Skill.Tags) != 2 {
		tags := 0
		if m.Skill != nil {
			tags = len(m.Skill.Tags)
		}
		t.Errorf("skill.tags = %d, want 2", tags)
	}
}

func TestBuildManifest_Defaults(t *testing.T) {
	skill := importedSkill{name: "test"}
	m := buildManifest(skill, "", t.TempDir(), "", "")
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
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}

	skill := importedSkill{name: "myapp", description: "A CLI tool"}
	m := buildManifest(skill, "user", dir, "", "")

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
	m := buildManifest(skill, "user", t.TempDir(), "", "")
	want := filepath.Join("skills/fastmail", "SKILL.md")
	if m.Skill.Entry != want {
		t.Errorf("skill.entry = %q, want %q", m.Skill.Entry, want)
	}
}

func TestBuildManifest_TriggersGoToSkillTags(t *testing.T) {
	tags := make([]string, 30)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag-%d", i)
	}
	skill := importedSkill{name: "test", tags: tags}
	m := buildManifest(skill, "", t.TempDir(), "", "")
	// Triggers go to both skill.tags and keywords (capped at 10)
	if len(m.Keywords) == 0 {
		t.Error("keywords should be populated with triggers for search")
	}
	if len(m.Keywords) > 10 {
		t.Errorf("keywords = %d, want <= 10 (should be capped)", len(m.Keywords))
	}
	if m.Skill == nil || len(m.Skill.Tags) == 0 {
		t.Error("skill.tags should be populated with triggers")
	}
	if len(m.Skill.Tags) > 10 {
		t.Errorf("skill.tags = %d, want <= 10 (should be capped)", len(m.Skill.Tags))
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

// --- applyTypeOverride tests ---

func TestApplyTypeOverride_NoFlag(t *testing.T) {
	m := &manifest.Manifest{Type: manifest.TypeSkill, Name: "@test/pkg"}
	old := flagInitType
	flagInitType = ""
	defer func() { flagInitType = old }()

	applyTypeOverride(m)
	if m.Type != manifest.TypeSkill {
		t.Errorf("type = %q, want skill (no override)", m.Type)
	}
}

func TestApplyTypeOverride_ForceCLI(t *testing.T) {
	m := &manifest.Manifest{
		Type: manifest.TypeSkill,
		Name: "@test/myapp",
	}
	old := flagInitType
	flagInitType = "cli"
	defer func() { flagInitType = old }()

	applyTypeOverride(m)
	if m.Type != manifest.TypeCLI {
		t.Errorf("type = %q, want cli", m.Type)
	}
	if m.CLI == nil {
		t.Fatal("cli section should be auto-created")
	}
	if m.CLI.Binary != "myapp" {
		t.Errorf("cli.binary = %q, want myapp", m.CLI.Binary)
	}
	if m.CLI.Verify != "myapp --version" {
		t.Errorf("cli.verify = %q, want 'myapp --version'", m.CLI.Verify)
	}
}

func TestApplyTypeOverride_ForceSkill(t *testing.T) {
	// Force a CLI-detected project back to skill
	m := &manifest.Manifest{
		Type: manifest.TypeCLI,
		Name: "@test/pkg",
		CLI:  &manifest.CLISpec{Binary: "pkg"},
	}
	old := flagInitType
	flagInitType = "skill"
	defer func() { flagInitType = old }()

	applyTypeOverride(m)
	if m.Type != manifest.TypeSkill {
		t.Errorf("type = %q, want skill (forced)", m.Type)
	}
	// CLI section still present (override only changes type, doesn't remove sections)
}

func TestApplyTypeOverride_CLIPreservesExisting(t *testing.T) {
	// If CLI section already exists, don't overwrite it
	m := &manifest.Manifest{
		Type: manifest.TypeSkill,
		Name: "@test/myapp",
		CLI:  &manifest.CLISpec{Binary: "custom-bin", Verify: "custom-bin check"},
	}
	old := flagInitType
	flagInitType = "cli"
	defer func() { flagInitType = old }()

	applyTypeOverride(m)
	if m.CLI.Binary != "custom-bin" {
		t.Errorf("cli.binary = %q, want custom-bin (should preserve existing)", m.CLI.Binary)
	}
}

// --- detectProjectType extended tests ---

func TestDetectProjectType_Extended(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  manifest.PackageType
	}{
		{
			name: "go_cli_with_cmd",
			setup: func(dir string) {
				writeFixture(t, dir, "go.mod", "module test\ngo 1.24\n")
				os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755)
			},
			want: manifest.TypeCLI,
		},
		{
			name: "go_library_no_cmd",
			setup: func(dir string) {
				writeFixture(t, dir, "go.mod", "module test\n")
				// No cmd/ directory → not a CLI
			},
			want: manifest.TypeSkill,
		},
		{
			name: "rust_cli",
			setup: func(dir string) {
				writeFixture(t, dir, "Cargo.toml", "[package]\nname = \"mycli\"\n")
				writeFixture(t, dir, "src/main.rs", "fn main() {}\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "rust_library",
			setup: func(dir string) {
				writeFixture(t, dir, "Cargo.toml", "[package]\nname = \"mylib\"\n")
				writeFixture(t, dir, "src/lib.rs", "pub fn hello() {}\n")
				// No src/main.rs → not a CLI
			},
			want: manifest.TypeSkill,
		},
		{
			name: "python_setup_py",
			setup: func(dir string) {
				writeFixture(t, dir, "setup.py", "from setuptools import setup\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "goreleaser_yaml",
			setup: func(dir string) {
				writeFixture(t, dir, ".goreleaser.yaml", "builds:\n  - main: ./cmd/app\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "goreleaser_yml",
			setup: func(dir string) {
				writeFixture(t, dir, ".goreleaser.yml", "builds:\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "go_with_makefile",
			setup: func(dir string) {
				writeFixture(t, dir, "go.mod", "module test\n")
				writeFixture(t, dir, "Makefile", "build:\n\tgo build\n")
			},
			want: manifest.TypeCLI,
		},
		{
			name: "empty_dir",
			setup: func(dir string) {},
			want:  manifest.TypeSkill,
		},
		{
			name: "skill_with_references",
			setup: func(dir string) {
				writeFixture(t, dir, "SKILL.md", "---\nname: test\n---\n")
				writeFixture(t, dir, "references/guide.md", "# Guide\n")
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

// --- deduplicateSkills tests ---

func TestDeduplicateSkills(t *testing.T) {
	skills := []importedSkill{
		{name: "a", dir: "dir1"},
		{name: "b", dir: "dir2"},
		{name: "a", dir: "dir3"}, // duplicate name
		{name: "c", dir: "dir4"},
		{name: "b", dir: "dir5"}, // duplicate name
	}
	got := deduplicateSkills(skills)
	if len(got) != 3 {
		t.Fatalf("dedup count = %d, want 3", len(got))
	}
	// First occurrence wins
	if got[0].dir != "dir1" || got[1].dir != "dir2" || got[2].dir != "dir4" {
		t.Errorf("dedup = %v, want dirs [dir1 dir2 dir4]", got)
	}
}

func TestDeduplicateSkills_Empty(t *testing.T) {
	got := deduplicateSkills(nil)
	if len(got) != 0 {
		t.Errorf("dedup(nil) = %v, want empty", got)
	}
}

func TestDeduplicateSkills_AllUnique(t *testing.T) {
	skills := []importedSkill{{name: "a"}, {name: "b"}, {name: "c"}}
	got := deduplicateSkills(skills)
	if len(got) != 3 {
		t.Errorf("dedup(all unique) = %d, want 3", len(got))
	}
}

// --- end-to-end: CLI project import shape ---

func TestImport_CLIProject_GeneratesCorrectShape(t *testing.T) {
	dir := t.TempDir()
	// Simulate a Go CLI project with SKILL.md in subdirectory
	writeFixture(t, dir, "go.mod", "module github.com/example/mycli\n\ngo 1.24\n")
	os.MkdirAll(filepath.Join(dir, "cmd", "mycli"), 0o755)
	writeFixture(t, dir, "skills/mycli/SKILL.md", "---\nname: mycli\ndescription: My CLI tool\ntriggers:\n  - /mycli\n  - run mycli\n---\n# Usage\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("detectImportFormat: %v", err)
	}
	if det.format != importFormatSingleSkill {
		t.Fatalf("format = %v, want single-skill", det.format)
	}
	if len(det.skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(det.skills))
	}

	skill := det.skills[0]
	m := buildManifest(skill, "user", dir, "", "")

	// Should be CLI type
	if m.Type != manifest.TypeCLI {
		t.Errorf("type = %q, want cli", m.Type)
	}
	// CLI section should exist
	if m.CLI == nil {
		t.Fatal("cli section is nil")
	}
	if m.CLI.Binary != "mycli" {
		t.Errorf("cli.binary = %q, want mycli", m.CLI.Binary)
	}
	// Skill entry should include subdirectory path
	if m.Skill == nil || m.Skill.Entry != filepath.Join("skills/mycli", "SKILL.md") {
		entry := ""
		if m.Skill != nil {
			entry = m.Skill.Entry
		}
		t.Errorf("skill.entry = %q, want skills/mycli/SKILL.md", entry)
	}
	// Description from SKILL.md
	if m.Description != "My CLI tool" {
		t.Errorf("description = %q, want 'My CLI tool'", m.Description)
	}
	// Triggers go to both skill.tags and keywords
	if len(m.Keywords) != 2 {
		t.Errorf("keywords = %v, want 2", m.Keywords)
	}
	if m.Skill == nil || len(m.Skill.Tags) != 2 {
		tags := 0
		if m.Skill != nil {
			tags = len(m.Skill.Tags)
		}
		t.Errorf("skill.tags = %d, want 2", tags)
	}
}

func TestImport_PureSkillProject_NoCliSection(t *testing.T) {
	dir := t.TempDir()
	// Pure skill — no go.mod, no cmd/
	writeFixture(t, dir, "SKILL.md", "---\nname: my-skill\ndescription: A pure skill\n---\n# Content\n")

	det, err := detectImportFormat(dir)
	if err != nil {
		t.Fatalf("detectImportFormat: %v", err)
	}

	skill := det.skills[0]
	m := buildManifest(skill, "user", dir, "", "")

	if m.Type != manifest.TypeSkill {
		t.Errorf("type = %q, want skill", m.Type)
	}
	if m.CLI != nil {
		t.Errorf("cli section should be nil for pure skill, got %+v", m.CLI)
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

// --- detectCLIFromGoreleaser tests ---

func TestDetectCLIFromGoreleaser_Yaml(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".goreleaser.yaml", `
builds:
  - binary: fastmail
    main: ./cmd/fastmail
`)
	binary, ok := detectCLIFromGoreleaser(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if binary != "fastmail" {
		t.Errorf("binary = %q, want fastmail", binary)
	}
}

func TestDetectCLIFromGoreleaser_Yml(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".goreleaser.yml", `
builds:
  - binary: myapp
`)
	binary, ok := detectCLIFromGoreleaser(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if binary != "myapp" {
		t.Errorf("binary = %q, want myapp", binary)
	}
}

func TestDetectCLIFromGoreleaser_MultipleBuilds(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".goreleaser.yaml", `
builds:
  - id: primary
    binary: fastmail
  - id: alias
    binary: fm
`)
	binary, ok := detectCLIFromGoreleaser(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if binary != "fastmail" {
		t.Errorf("binary = %q, want fastmail (first build)", binary)
	}
}

func TestDetectCLIFromGoreleaser_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, ok := detectCLIFromGoreleaser(dir)
	if ok {
		t.Fatal("expected ok=false when no goreleaser config")
	}
}

func TestDetectCLIFromGoreleaser_EmptyBuilds(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".goreleaser.yaml", "version: 2\n")
	_, ok := detectCLIFromGoreleaser(dir)
	if ok {
		t.Fatal("expected ok=false when no builds section")
	}
}

// --- detectInstallSpec tests ---

func TestDetectInstallSpec_WithScriptAndRepo(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "scripts/install.sh", "#!/bin/sh\necho install")

	spec := detectInstallSpec(dir, "https://github.com/biao29/fastmail-cli", "")
	if spec == nil {
		t.Fatal("expected non-nil install spec")
	}
	if spec.Source != "github:biao29/fastmail-cli" {
		t.Errorf("source = %q, want github:biao29/fastmail-cli", spec.Source)
	}
	if spec.Script == "" {
		t.Fatal("expected script URL to be set")
	}
	if !strings.Contains(spec.Script, "raw.githubusercontent.com/biao29/fastmail-cli") {
		t.Errorf("script = %q, should contain raw.githubusercontent URL", spec.Script)
	}
}

func TestDetectInstallSpec_WithScriptAndRepoRelDir(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "scripts/install.sh", "#!/bin/sh\necho install")

	spec := detectInstallSpec(dir, "https://github.com/owner/monorepo", "packages/mycli")
	if spec == nil {
		t.Fatal("expected non-nil install spec")
	}
	// Script URL should include the repo-relative subdir
	want := "packages/mycli/scripts/install.sh"
	if !strings.Contains(spec.Script, want) {
		t.Errorf("script = %q, should contain %q for workspace member", spec.Script, want)
	}
}

func TestDetectInstallSpec_NoScript(t *testing.T) {
	dir := t.TempDir()

	// No scripts/install.sh → no install method → nil
	spec := detectInstallSpec(dir, "https://github.com/owner/repo", "")
	if spec != nil {
		t.Errorf("expected nil install spec when no install method, got %+v", spec)
	}
}

func TestDetectInstallSpec_NoRepo(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "scripts/install.sh", "#!/bin/sh\n")

	spec := detectInstallSpec(dir, "", "")
	if spec != nil {
		t.Errorf("expected nil install spec when no repo URL, got %+v", spec)
	}
}

func TestDetectInstallSpec_NonGitHub(t *testing.T) {
	dir := t.TempDir()

	spec := detectInstallSpec(dir, "https://gitlab.com/owner/repo", "")
	if spec != nil {
		t.Errorf("expected nil install spec for non-GitHub repo, got %+v", spec)
	}
}

// --- githubOwnerRepo tests ---

func TestGithubOwnerRepo(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/biao29/fastmail-cli", "biao29/fastmail-cli"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo/", "owner/repo"},
		{"https://gitlab.com/owner/repo", ""},
		{"", ""},
		{"https://github.com/owner", ""},
		{"https://github.com/owner/repo/extra", ""},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := githubOwnerRepo(tt.url)
			if got != tt.want {
				t.Errorf("githubOwnerRepo(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --- buildManifest CLI + goreleaser integration ---

func TestBuildManifest_CLIWithGoreleaser(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module github.com/example/cli\n\ngo 1.24\n")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, ".goreleaser.yaml", `
builds:
  - binary: myapp
    main: ./cmd/myapp
`)
	writeFixture(t, dir, "scripts/install.sh", "#!/bin/sh\necho install")

	repo := "https://github.com/user/myapp-cli"
	skill := importedSkill{name: "myapp", description: "A CLI tool"}
	m := buildManifest(skill, "user", dir, repo, "")

	if m.Type != manifest.TypeCLI {
		t.Errorf("type = %q, want cli", m.Type)
	}
	if m.CLI == nil {
		t.Fatal("cli section is nil")
	}
	// Binary should come from goreleaser, not directory name
	if m.CLI.Binary != "myapp" {
		t.Errorf("cli.binary = %q, want myapp (from goreleaser)", m.CLI.Binary)
	}
	// Install should be auto-populated
	if m.Install == nil {
		t.Fatal("install section should be auto-populated")
	}
	if m.Install.Source != "github:user/myapp-cli" {
		t.Errorf("install.source = %q, want github:user/myapp-cli", m.Install.Source)
	}
	if m.Install.Script == "" {
		t.Error("install.script should be set (scripts/install.sh exists)")
	}
}

func TestBuildManifest_CLIGoreleaserOverridesName(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module github.com/example/cli\n\ngo 1.24\n")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Binary name differs from directory/skill name
	writeFixture(t, dir, ".goreleaser.yaml", `
builds:
  - binary: different-name
`)

	skill := importedSkill{name: "myskill", description: "A CLI"}
	m := buildManifest(skill, "user", dir, "", "")

	if m.CLI == nil {
		t.Fatal("cli section is nil")
	}
	if m.CLI.Binary != "different-name" {
		t.Errorf("cli.binary = %q, want different-name (from goreleaser)", m.CLI.Binary)
	}
	if m.CLI.Verify != "different-name --version" {
		t.Errorf("cli.verify = %q, want 'different-name --version'", m.CLI.Verify)
	}
}
