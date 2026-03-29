package installer

import (
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected SourceFormat
	}{
		{"empty", "", FormatGitHubRaw},
		{"raw markdown", "# My Skill\n\nDoes things", FormatGitHubRaw},
		{"ctx-native", "---\nname: my-skill\ndescription: test\n---\n# Content", FormatCtxNative},
		{"clawhub", "---\nname: test\nmetadata:\n  openclaw:\n    requires: []\n---\nBody", FormatClawHub},
		{"skillsgate", "---\ncategories: [ai]\ncapabilities: [review]\n---\nBody", FormatSkillsGate},
		{"unknown frontmatter", "---\nrandom: stuff\n---\nBody", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.content)
			if got != tt.expected {
				t.Errorf("DetectFormat() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractFromMarkdown(t *testing.T) {
	content := "# Code Review Helper\n\nReviews PRs automatically\n\n## Usage\n\n## Configuration\n"
	name, desc, triggers := ExtractFromMarkdown(content)

	if name != "Code Review Helper" {
		t.Errorf("name = %q, want %q", name, "Code Review Helper")
	}
	if desc != "Reviews PRs automatically" {
		t.Errorf("description = %q, want %q", desc, "Reviews PRs automatically")
	}
	if len(triggers) != 2 {
		t.Errorf("triggers = %v, want 2 items", triggers)
	}
}

func TestExtractFromMarkdown_NoHeading(t *testing.T) {
	name, desc, _ := ExtractFromMarkdown("Just some text")
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
	if desc != "" {
		t.Errorf("description = %q, want empty (no heading means no description capture)", desc)
	}
}

func TestNormalize_CtxNative(t *testing.T) {
	content := "---\nname: my-skill\ndescription: test\n---\n# Content"
	result := Normalize(content)
	if result.NeedsEnrichment {
		t.Error("ctx-native should not need enrichment")
	}
	if result.SourceFormat != FormatCtxNative {
		t.Errorf("format = %q, want ctx-native", result.SourceFormat)
	}
}

func TestNormalize_GitHubRaw(t *testing.T) {
	content := "# My Awesome Skill\n\nDoes great things\n"
	result := Normalize(content)
	if !result.NeedsEnrichment {
		t.Error("github-raw should need enrichment")
	}
	if result.AddedFields["name"] != "My Awesome Skill" {
		t.Errorf("name = %q, want %q", result.AddedFields["name"], "My Awesome Skill")
	}
	if result.AddedFields["description"] != "Does great things" {
		t.Errorf("description = %q, want %q", result.AddedFields["description"], "Does great things")
	}
	if result.AddedFields["compatibility"] == "" {
		t.Error("should add default compatibility")
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	content := "# Skill\n\nDescription"
	r1 := Normalize(content)
	r2 := Normalize(content)
	if r1.OriginalHash != r2.OriginalHash {
		t.Error("normalize should be idempotent")
	}
	if len(r1.AddedFields) != len(r2.AddedFields) {
		t.Error("added fields should be same")
	}
}

func TestMergeEnrichment_NoEnrichment(t *testing.T) {
	content := "---\nname: test\n---\n# Hi"
	result := &EnrichmentResult{NeedsEnrichment: false}
	merged := MergeEnrichment(content, result)
	if merged != content {
		t.Error("should return original when no enrichment needed")
	}
}

func TestMergeEnrichment_AddsFrontmatter(t *testing.T) {
	content := "# My Skill\n\nContent here"
	result := &EnrichmentResult{
		NeedsEnrichment: true,
		AddedFields:     map[string]string{"name": "My Skill", "compatibility": "claude,cursor"},
	}
	merged := MergeEnrichment(content, result)
	if !strings.HasPrefix(merged, "---\n") {
		t.Error("should add frontmatter")
	}
	if !strings.Contains(merged, "name:") {
		t.Error("should contain name field")
	}
	if !strings.Contains(merged, "# My Skill") {
		t.Error("should preserve original content")
	}
}
