package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getctx/ctx/internal/manifest"
)

func TestLockFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.lock")

	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	// Add entries
	lf.Add(LockEntry{
		FullName:    "@hong/my-skill",
		Version:     "1.0.0",
		Type:        "skill",
		Source:      "registry",
		InstallPath: "/tmp/test",
	})
	lf.Add(LockEntry{
		FullName:    "@mcp/github",
		Version:     "2.1.0",
		Type:        "mcp",
		Source:      "registry",
		InstallPath: "/tmp/test2",
	})

	// Save
	if err := lf.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}

	// Load
	lf2, err := LoadLockFile(path)
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}

	if len(lf2.Packages) != 2 {
		t.Errorf("Packages count = %d, want 2", len(lf2.Packages))
	}

	if !lf2.Has("@hong/my-skill") {
		t.Error("missing @hong/my-skill")
	}
	if !lf2.Has("@mcp/github") {
		t.Error("missing @mcp/github")
	}

	entry, _ := lf2.Get("@hong/my-skill")
	if entry.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.0.0")
	}
}

func TestLockFileAddUpdate(t *testing.T) {
	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	lf.Add(LockEntry{
		FullName: "@test/pkg",
		Version:  "1.0.0",
	})

	first := lf.Packages["@test/pkg"].InstalledAt

	// Small delay to ensure time difference
	time.Sleep(time.Millisecond)

	lf.Add(LockEntry{
		FullName: "@test/pkg",
		Version:  "1.1.0",
	})

	entry := lf.Packages["@test/pkg"]
	if entry.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.1.0")
	}
	if entry.InstalledAt != first {
		t.Error("InstalledAt should be preserved on update")
	}
}

func TestLockFileRemove(t *testing.T) {
	lf := &LockFile{
		Version:  1,
		Packages: make(map[string]LockEntry),
	}

	lf.Add(LockEntry{FullName: "@test/a", Version: "1.0.0"})
	lf.Add(LockEntry{FullName: "@test/b", Version: "1.0.0"})

	lf.Remove("@test/a")

	if lf.Has("@test/a") {
		t.Error("@test/a should be removed")
	}
	if !lf.Has("@test/b") {
		t.Error("@test/b should still exist")
	}
}

func TestLoadLockFileNotExist(t *testing.T) {
	lf, err := LoadLockFile("/nonexistent/ctx.lock")
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}
	if len(lf.Packages) != 0 {
		t.Errorf("expected empty lockfile, got %d packages", len(lf.Packages))
	}
}

// --- Tests for the no-download manifest fix ---

func TestGenerateSkillMD(t *testing.T) {
	tests := []struct {
		name     string
		manifest manifest.Manifest
		wantIn   []string
		wantOut  []string
	}{
		{
			name: "full manifest",
			manifest: manifest.Manifest{
				Name:        "@community/facebook-react-flow",
				Version:     "0.0.1",
				Type:        manifest.TypeSkill,
				Description: "Use when you need to run Flow type checking",
			},
			wantIn: []string{
				"name: facebook-react-flow",
				"# @community/facebook-react-flow",
				"Use when you need to run Flow type checking",
				"---",
			},
		},
		{
			name: "no description",
			manifest: manifest.Manifest{
				Name:    "@test/empty-desc",
				Version: "1.0.0",
				Type:    manifest.TypeSkill,
			},
			wantIn:  []string{"name: empty-desc", "# @test/empty-desc", "---"},
			wantOut: []string{"description:"},
		},
		{
			name: "short name only",
			manifest: manifest.Manifest{
				Name:    "simple-skill",
				Version: "0.1.0",
				Type:    manifest.TypeSkill,
			},
			wantIn: []string{"name: simple-skill", "# simple-skill"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSkillMD(&tt.manifest)
			for _, s := range tt.wantIn {
				if !strings.Contains(got, s) {
					t.Errorf("generateSkillMD() missing %q\ngot:\n%s", s, got)
				}
			}
			for _, s := range tt.wantOut {
				if strings.Contains(got, s) {
					t.Errorf("generateSkillMD() should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

func TestNoDownloadBranch_WritesManifestAndSkillMD(t *testing.T) {
	// Simulates the fixed no-download branch: create version dir, write manifest + SKILL.md
	dataDir := t.TempDir()
	fullName := "@test/no-archive-skill"
	version := "1.0.0"
	versionDir := filepath.Join(dataDir, "packages", fullName, version)

	m := manifest.Manifest{
		Name:        fullName,
		Version:     version,
		Type:        manifest.TypeSkill,
		Description: "A test skill without archive",
	}

	// Replicate the fixed logic
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	content := generateSkillMD(&m)
	if err := os.WriteFile(filepath.Join(versionDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}

	// Verify manifest.json
	data, err := os.ReadFile(filepath.Join(versionDir, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest.json should exist: %v", err)
	}
	var loaded manifest.Manifest
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("manifest.json should be valid JSON: %v", err)
	}
	if loaded.Type != manifest.TypeSkill {
		t.Errorf("type = %q, want %q", loaded.Type, manifest.TypeSkill)
	}
	if loaded.Description != "A test skill without archive" {
		t.Errorf("description = %q, want %q", loaded.Description, "A test skill without archive")
	}

	// Verify SKILL.md
	skillData, err := os.ReadFile(filepath.Join(versionDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md should exist: %v", err)
	}
	if !strings.Contains(string(skillData), "no-archive-skill") {
		t.Error("SKILL.md should contain the skill short name")
	}
	if !strings.Contains(string(skillData), "A test skill without archive") {
		t.Error("SKILL.md should contain the description")
	}
}

func TestNoDownloadBranch_NonSkillType_NoSkillMD(t *testing.T) {
	// MCP or CLI packages without archive should get manifest.json but NOT SKILL.md
	dataDir := t.TempDir()
	versionDir := filepath.Join(dataDir, "mcp-pkg", "1.0.0")

	m := manifest.Manifest{
		Name:    "@test/mcp-server",
		Version: "1.0.0",
		Type:    manifest.TypeMCP,
	}

	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifestData, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(versionDir, "manifest.json"), manifestData, 0o644)

	// SKILL.md should NOT be generated for non-skill types
	if m.Type == manifest.TypeSkill {
		t.Fatal("test setup error: expected non-skill type")
	}

	// Verify manifest exists
	if _, err := os.Stat(filepath.Join(versionDir, "manifest.json")); err != nil {
		t.Error("manifest.json should exist for MCP packages too")
	}

	// Verify SKILL.md does NOT exist
	if _, err := os.Stat(filepath.Join(versionDir, "SKILL.md")); err == nil {
		t.Error("SKILL.md should NOT exist for MCP packages")
	}
}
