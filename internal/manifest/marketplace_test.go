package manifest

import (
	"strings"
	"testing"
)

func TestParseMarketplaceJSON_BaoyuFormat(t *testing.T) {
	input := `{
  "name": "baoyu-skills",
  "owner": {"name": "Jim Liu", "email": "jim@example.com"},
  "metadata": {"description": "Skills by Baoyu", "version": "1.89.0"},
  "plugins": [{
    "name": "baoyu-skills",
    "description": "Content generation tools",
    "source": "./",
    "strict": true,
    "skills": ["./skills/baoyu-translate", "./skills/baoyu-imagine"]
  }]
}`
	mf, err := ParseMarketplaceJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mf.Name != "baoyu-skills" {
		t.Errorf("name = %q, want %q", mf.Name, "baoyu-skills")
	}
	if mf.Owner == nil || mf.Owner.Name != "Jim Liu" {
		t.Errorf("owner.name = %v, want Jim Liu", mf.Owner)
	}
	if mf.Metadata == nil || mf.Metadata.Version != "1.89.0" {
		t.Errorf("metadata.version = %v, want 1.89.0", mf.Metadata)
	}
	if len(mf.Plugins) != 1 {
		t.Fatalf("plugins count = %d, want 1", len(mf.Plugins))
	}
	if len(mf.Plugins[0].Skills) != 2 {
		t.Errorf("skills count = %d, want 2", len(mf.Plugins[0].Skills))
	}
	if !mf.Plugins[0].Strict {
		t.Error("strict = false, want true")
	}
}

func TestParseMarketplaceJSON_AnthropicFormat(t *testing.T) {
	input := `{
  "name": "anthropic-agent-skills",
  "owner": {"name": "Keith", "email": "k@anthropic.com"},
  "metadata": {"description": "Anthropic skills", "version": "1.0.0"},
  "plugins": [
    {"name": "document-skills", "description": "Docs", "source": "./", "strict": false, "skills": ["./skills/xlsx", "./skills/docx"]},
    {"name": "example-skills", "description": "Examples", "source": "./", "strict": false, "skills": ["./skills/algorithmic-art"]},
    {"name": "claude-api", "description": "API", "source": "./", "strict": false, "skills": ["./skills/claude-api"]}
  ]
}`
	mf, err := ParseMarketplaceJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mf.Plugins) != 3 {
		t.Fatalf("plugins count = %d, want 3", len(mf.Plugins))
	}

	paths := MarketplaceSkillPaths(mf)
	if len(paths) != 4 {
		t.Fatalf("paths count = %d, want 4", len(paths))
	}
	want := []string{"skills/xlsx", "skills/docx", "skills/algorithmic-art", "skills/claude-api"}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestParseMarketplaceJSON_EmptyPlugins(t *testing.T) {
	input := `{"name": "empty", "plugins": []}`
	mf, err := ParseMarketplaceJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mf.Plugins) != 0 {
		t.Errorf("plugins count = %d, want 0", len(mf.Plugins))
	}
	paths := MarketplaceSkillPaths(mf)
	if len(paths) != 0 {
		t.Errorf("paths count = %d, want 0", len(paths))
	}
}

func TestParseMarketplaceJSON_InvalidJSON(t *testing.T) {
	_, err := ParseMarketplaceJSON(strings.NewReader(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMarketplaceSkillPaths_Dedup(t *testing.T) {
	mf := &MarketplaceFile{
		Plugins: []MarketplacePlugin{
			{Skills: []string{"./skills/a", "./skills/b"}},
			{Skills: []string{"./skills/a", "./skills/c"}}, // "a" duplicated
		},
	}
	paths := MarketplaceSkillPaths(mf)
	if len(paths) != 3 {
		t.Fatalf("paths count = %d, want 3 (deduped)", len(paths))
	}
}

func TestMarketplaceSkillPaths_CleansPaths(t *testing.T) {
	mf := &MarketplaceFile{
		Plugins: []MarketplacePlugin{
			{Skills: []string{"./skills/a/", "skills/b", "./", "."}},
		},
	}
	paths := MarketplaceSkillPaths(mf)
	// "./" and "." should be filtered out
	if len(paths) != 2 {
		t.Fatalf("paths = %v, want [skills/a skills/b]", paths)
	}
	if paths[0] != "skills/a" || paths[1] != "skills/b" {
		t.Errorf("paths = %v, want [skills/a skills/b]", paths)
	}
}
