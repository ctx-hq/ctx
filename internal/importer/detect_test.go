package importer

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestDetectLayout(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string)
		wantFmt Format
		wantN   int
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
			wantFmt: FormatMarketplace,
			wantN:   2,
		},
		{
			name: "codex_curated",
			setup: func(dir string) {
				writeFixture(t, dir, "skills/.curated/gc/SKILL.md", "---\nname: gc\ndescription: Git commit\n---\n")
				writeFixture(t, dir, "skills/.curated/review/SKILL.md", "---\nname: review\ndescription: Review\n---\n")
			},
			wantFmt: FormatCodex,
			wantN:   2,
		},
		{
			name: "codex_system",
			setup: func(dir string) {
				writeFixture(t, dir, "skills/.system/creator/SKILL.md", "---\nname: creator\n---\n")
			},
			wantFmt: FormatCodex,
			wantN:   1,
		},
		{
			name: "single_skill_with_frontmatter",
			setup: func(dir string) {
				writeFixture(t, dir, "SKILL.md", "---\nname: my-skill\ndescription: A skill\n---\n# Content\n")
			},
			wantFmt: FormatSingleSkill,
			wantN:   1,
		},
		{
			name: "flat_skill_dirs",
			setup: func(dir string) {
				writeFixture(t, dir, "gc/SKILL.md", "---\nname: gc\n---\n")
				writeFixture(t, dir, "review/SKILL.md", "---\nname: review\n---\n")
			},
			wantFmt: FormatFlatSkills,
			wantN:   2,
		},
		{
			name: "nested_skill_dirs",
			setup: func(dir string) {
				writeFixture(t, dir, "engineering/gc/SKILL.md", "---\nname: gc\n---\n")
				writeFixture(t, dir, "writing/translate/SKILL.md", "---\nname: translate\n---\n")
			},
			wantFmt: FormatNestedSkills,
			wantN:   2,
		},
		{
			name: "bare_markdown",
			setup: func(dir string) {
				writeFixture(t, dir, "minutes.md", "# Minutes\nNo frontmatter here.\n")
			},
			wantFmt: FormatBareMarkdown,
			wantN:   1,
		},
		{
			name: "unknown_empty_repo",
			setup: func(dir string) {
				writeFixture(t, dir, "README.md", "# Just a readme\n")
			},
			wantFmt: FormatUnknown,
			wantN:   0,
		},
		{
			name: "readme_only_not_bare",
			setup: func(dir string) {
				writeFixture(t, dir, "README.md", "# Project\nSome content\n")
				writeFixture(t, dir, "CHANGELOG.md", "# Changelog\n")
			},
			wantFmt: FormatUnknown,
			wantN:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			det, err := DetectLayout(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if det.Format != tt.wantFmt {
				t.Errorf("format = %v, want %v", det.Format, tt.wantFmt)
			}
			if len(det.Skills) != tt.wantN {
				t.Errorf("skills count = %d, want %d", len(det.Skills), tt.wantN)
			}
		})
	}
}

func TestDetectLayout_MarketplaceNonexistentDir(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude-plugin/marketplace.json", `{
		"name": "test", "plugins": [{
			"name": "p1", "skills": ["./skills/exists", "./skills/missing"]
		}]
	}`)
	writeFixture(t, dir, "skills/exists/SKILL.md", "---\nname: exists\n---\n")

	det, err := DetectLayout(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if det.Format != FormatMarketplace {
		t.Errorf("format = %v, want marketplace", det.Format)
	}
	if len(det.Skills) != 1 {
		t.Errorf("skills = %d, want 1 (missing dir skipped)", len(det.Skills))
	}
}

func TestDetectLayout_SkipInternalDir(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "internal/skills/SKILL.md", "---\nname: fastmail\ndescription: CLI tool\n---\n")
	writeFixture(t, dir, "skills/fastmail/SKILL.md", "---\nname: fastmail\ndescription: CLI tool\n---\n")

	det, err := DetectLayout(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if det.Format != FormatSingleSkill {
		t.Errorf("format = %v, want single-skill (internal/ excluded + dedup)", det.Format)
	}
	if len(det.Skills) != 1 {
		t.Errorf("skills = %d, want 1 (deduped)", len(det.Skills))
	}
}

func TestDetectLayout_FlatSkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "gc/SKILL.md", "---\nname: gc\n---\n")
	writeFixture(t, dir, ".hidden/SKILL.md", "---\nname: hidden\n---\n")
	writeFixture(t, dir, "node_modules/SKILL.md", "---\nname: nm\n---\n")

	det, err := DetectLayout(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(det.Skills) != 1 {
		t.Errorf("skills = %d, want 1 (hidden and node_modules skipped)", len(det.Skills))
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
		got := ContainsExcludedDir(tt.path)
		if got != tt.want {
			t.Errorf("ContainsExcludedDir(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDeduplicateSkills(t *testing.T) {
	skills := []Skill{
		{Name: "a", Dir: "dir1"},
		{Name: "b", Dir: "dir2"},
		{Name: "a", Dir: "dir3"},
		{Name: "c", Dir: "dir4"},
		{Name: "b", Dir: "dir5"},
	}
	got := DeduplicateSkills(skills)
	if len(got) != 3 {
		t.Fatalf("dedup count = %d, want 3", len(got))
	}
	if got[0].Dir != "dir1" || got[1].Dir != "dir2" || got[2].Dir != "dir4" {
		t.Errorf("dedup = %v, want dirs [dir1 dir2 dir4]", got)
	}
}

func TestInferMemberGlobs(t *testing.T) {
	tests := []struct {
		name   string
		skills []Skill
		want   []string
	}{
		{
			name:   "common_prefix",
			skills: []Skill{{Dir: "skills/a"}, {Dir: "skills/b"}, {Dir: "skills/c"}},
			want:   []string{"skills/*"},
		},
		{
			name:   "root_level",
			skills: []Skill{{Dir: "a"}, {Dir: "b"}},
			want:   []string{"*"},
		},
		{
			name:   "empty",
			skills: nil,
			want:   []string{"*"},
		},
		{
			name:   "two_levels",
			skills: []Skill{{Dir: "eng/gc/sub"}, {Dir: "eng/review/sub"}},
			want:   []string{"eng/*/*"},
		},
		{
			name:   "multi_prefix",
			skills: []Skill{{Dir: "engineering/gc"}, {Dir: "writing/translate"}},
			want:   []string{"engineering/*", "writing/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferMemberGlobs(tt.skills)
			if len(got) != len(tt.want) {
				t.Fatalf("InferMemberGlobs = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("InferMemberGlobs[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"my_skill_name", "my-skill-name"},
		{"---test---", "test"},
		{"CamelCase", "camelcase"},
		{"already-good", "already-good"},
	}
	for _, tt := range tests {
		got := Slugify(tt.in)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
