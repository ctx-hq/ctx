package installer

import (
	encJSON "encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinkRegistry_AddRemove(t *testing.T) {
	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}

	entry1 := LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: "/src/skill.md",
		Target: "/dst/skill.md",
	}
	entry2 := LinkEntry{
		Agent:  "cursor",
		Type:   LinkSymlink,
		Source: "/src/skill.md",
		Target: "/dst2/skill.md",
	}

	reg.Add("@hong/review", entry1)
	reg.Add("@hong/review", entry2)

	entries := reg.ForPackage("@hong/review")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Add should set CreatedAt
	for _, e := range entries {
		if e.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set automatically")
		}
	}

	// Remove returns all entries
	removed := reg.Remove("@hong/review")
	if len(removed) != 2 {
		t.Fatalf("Remove should return 2 entries, got %d", len(removed))
	}

	// Package should be gone
	if len(reg.ForPackage("@hong/review")) != 0 {
		t.Error("package should be removed from registry")
	}
}

func TestLinkRegistry_DeduplicateByTarget(t *testing.T) {
	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}

	entry := LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: "/src/v1/skill.md",
		Target: "/dst/skill.md",
	}
	reg.Add("@hong/review", entry)

	// Add same target again (e.g., after update)
	entry.Source = "/src/v2/skill.md"
	reg.Add("@hong/review", entry)

	entries := reg.ForPackage("@hong/review")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (deduplicated), got %d", len(entries))
	}
	if entries[0].Source != "/src/v2/skill.md" {
		t.Errorf("source should be updated to v2, got %q", entries[0].Source)
	}
}

func TestLinkRegistry_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links.json")

	// Temporarily override config dir
	origDir := os.Getenv("CTX_HOME")
	os.Setenv("CTX_HOME", dir)
	defer os.Setenv("CTX_HOME", origDir)

	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}

	now := time.Now().UTC().Truncate(time.Second)
	reg.Add("@hong/review", LinkEntry{
		Agent:     "claude",
		Type:      LinkSymlink,
		Source:    "/src/skill.md",
		Target:    "/dst/skill.md",
		CreatedAt: now,
	})

	// Save manually to the temp path
	reg.Version = linksFileVersion
	data, err := jsonMarshalIndent(reg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Load back
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var loaded LinkRegistry
	if err := jsonUnmarshal(data2, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	entries := loaded.Links["@hong/review"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after load, got %d", len(entries))
	}
	if entries[0].Agent != "claude" {
		t.Errorf("agent = %q, want %q", entries[0].Agent, "claude")
	}
}

func TestLinkRegistry_VerifyBrokenSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create a broken symlink
	target := filepath.Join(dir, "broken-link")
	os.Symlink("/nonexistent/path", target)

	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}
	reg.Add("@hong/review", LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: "/nonexistent/path",
		Target: target,
	})

	issues := reg.Verify()
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Problem != "broken_symlink" {
		t.Errorf("problem = %q, want %q", issues[0].Problem, "broken_symlink")
	}
}

func TestLinkRegistry_VerifyMissingTarget(t *testing.T) {
	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}
	reg.Add("@hong/review", LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: "/src/skill.md",
		Target: "/completely/nonexistent/path",
	})

	issues := reg.Verify()
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Problem != "missing_target" {
		t.Errorf("problem = %q, want %q", issues[0].Problem, "missing_target")
	}
}

func TestLinkRegistry_VerifyValidSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(dir, "source", "SKILL.md")
	os.MkdirAll(filepath.Dir(srcFile), 0o755)
	os.WriteFile(srcFile, []byte("# test"), 0o644)

	// Create valid symlink
	linkPath := filepath.Join(dir, "link")
	os.Symlink(filepath.Join(dir, "source"), linkPath)

	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}
	reg.Add("@hong/review", LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: filepath.Join(dir, "source"),
		Target: linkPath,
	})

	issues := reg.Verify()
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for valid symlink, got %d: %+v", len(issues), issues)
	}
}

func TestCleanupLinks_Symlink(t *testing.T) {
	dir := t.TempDir()

	// Create source and symlink
	srcDir := filepath.Join(dir, "source")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# test"), 0o644)

	// Create nested target dir and symlink
	targetParent := filepath.Join(dir, "agent", "skills")
	os.MkdirAll(targetParent, 0o755)
	linkPath := filepath.Join(targetParent, "review")
	os.Symlink(srcDir, linkPath)

	entries := []LinkEntry{
		{Agent: "claude", Type: LinkSymlink, Source: srcDir, Target: linkPath},
	}

	cleaned := CleanupLinks(entries)
	if cleaned != 1 {
		t.Errorf("cleaned = %d, want 1", cleaned)
	}

	// Symlink should be gone
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}
}

func TestCleanupLinks_CopiedDir(t *testing.T) {
	dir := t.TempDir()

	// Simulate a fallback copy with .ctx-managed marker
	copiedDir := filepath.Join(dir, "skills", "review")
	os.MkdirAll(copiedDir, 0o755)
	os.WriteFile(filepath.Join(copiedDir, "SKILL.md"), []byte("# test"), 0o644)
	os.WriteFile(filepath.Join(copiedDir, ".ctx-managed"), []byte("managed by ctx"), 0o644)

	entries := []LinkEntry{
		{Agent: "claude", Type: LinkSymlink, Source: "/src", Target: copiedDir},
	}

	cleaned := CleanupLinks(entries)
	if cleaned != 1 {
		t.Errorf("cleaned = %d, want 1", cleaned)
	}

	if _, err := os.Stat(copiedDir); !os.IsNotExist(err) {
		t.Error("copied dir with .ctx-managed should be removed")
	}
}

func TestCleanupLinks_AlreadyGone(t *testing.T) {
	entries := []LinkEntry{
		{Agent: "claude", Type: LinkSymlink, Source: "/src", Target: "/nonexistent"},
	}

	cleaned := CleanupLinks(entries)
	if cleaned != 0 {
		t.Errorf("cleaned = %d, want 0 (already gone)", cleaned)
	}
}

func TestLinkRegistry_EmptyRegistry(t *testing.T) {
	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}

	// Empty remove
	removed := reg.Remove("@nonexistent/pkg")
	if removed != nil {
		t.Error("Remove of nonexistent package should return nil")
	}

	// Empty verify
	issues := reg.Verify()
	if len(issues) != 0 {
		t.Error("Verify on empty registry should return no issues")
	}
}

func TestLinkRegistry_PathCompression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CTX_HOME", dir)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}
	reg.Add("@test/pkg", LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: filepath.Join(home, ".ctx", "packages", "@test", "pkg"),
		Target: filepath.Join(home, ".claude", "skills", "pkg"),
	})

	if err := reg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw JSON and verify paths use ~
	raw, err := os.ReadFile(filepath.Join(dir, "links.json"))
	if err != nil {
		t.Fatalf("read links.json: %v", err)
	}
	content := string(raw)

	if strings.Contains(content, home) {
		t.Errorf("links.json should not contain home dir %q, got:\n%s", home, content)
	}
	if !strings.Contains(content, "~/") && !strings.Contains(content, "~\\") {
		t.Errorf("links.json should contain ~/relative paths, got:\n%s", content)
	}
}

func TestLinkRegistry_PathExpansion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CTX_HOME", dir)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	// Write links.json with ~ paths directly
	data := []byte(`{
  "version": 1,
  "links": {
    "@test/pkg": [
      {
        "agent": "claude",
        "type": "symlink",
        "source": "~/.ctx/packages/@test/pkg",
        "target": "~/.claude/skills/pkg",
        "created_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
}`)
	if err := os.WriteFile(filepath.Join(dir, "links.json"), data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg, err := LoadLinks()
	if err != nil {
		t.Fatalf("LoadLinks: %v", err)
	}

	entries := reg.ForPackage("@test/pkg")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	expectedSource := filepath.Join(home, ".ctx", "packages", "@test", "pkg")
	if entries[0].Source != expectedSource {
		t.Errorf("Source = %q, want %q", entries[0].Source, expectedSource)
	}

	expectedTarget := filepath.Join(home, ".claude", "skills", "pkg")
	if entries[0].Target != expectedTarget {
		t.Errorf("Target = %q, want %q", entries[0].Target, expectedTarget)
	}
}

func TestLinkRegistry_NonHomePaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CTX_HOME", dir)

	reg := &LinkRegistry{
		Version: 1,
		Links:   make(map[string][]LinkEntry),
	}
	reg.Add("@test/pkg", LinkEntry{
		Agent:  "claude",
		Type:   LinkSymlink,
		Source: "/opt/ctx/packages/test",
		Target: "/opt/agents/skills/test",
	})

	if err := reg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Non-home paths should be unchanged
	raw, err := os.ReadFile(filepath.Join(dir, "links.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "/opt/ctx/packages/test") {
		t.Errorf("non-home path should be preserved, got:\n%s", content)
	}
}

// helpers
func jsonMarshalIndent(v any) ([]byte, error) {
	return encJSON.MarshalIndent(v, "", "  ")
}

func jsonUnmarshal(data []byte, v any) error {
	return encJSON.Unmarshal(data, v)
}
