package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func setupWorkspace(t *testing.T, root string, ctxYaml string, skills map[string]string) {
	t.Helper()
	writeFile(t, filepath.Join(root, FileName), ctxYaml)
	for dir, skillMD := range skills {
		writeFile(t, filepath.Join(root, dir, "SKILL.md"), skillMD)
	}
}

// --- ResolveMembers tests (covers all 7 repo patterns) ---

func TestResolveMembers_FlatSkillsDir(t *testing.T) {
	// Pattern: anthropic/skills, baoyu-skills — skills under skills/*
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skills", "translate", "SKILL.md"), "---\nname: translate\ndescription: t\n---\n")
	writeFile(t, filepath.Join(root, "skills", "comic", "SKILL.md"), "---\nname: comic\ndescription: c\n---\n")

	dirs, err := ResolveMembers(root, []string{"skills/*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("got %d dirs, want 2", len(dirs))
	}
}

func TestResolveMembers_RootLevel(t *testing.T) {
	// Pattern: dimillian, mattpocock — skills at root level
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "write-a-prd", "SKILL.md"), "---\nname: write-a-prd\ndescription: prd\n---\n")
	writeFile(t, filepath.Join(root, "tdd", "SKILL.md"), "---\nname: tdd\ndescription: tdd\n---\n")
	mkdirAll(t, filepath.Join(root, "docs")) // non-skill dir, no SKILL.md
	mkdirAll(t, filepath.Join(root, "scripts"))

	dirs, err := ResolveMembers(root, []string{"*"}, []string{"docs", "scripts"})
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("got %d dirs, want 2: %v", len(dirs), dirs)
	}
}

func TestResolveMembers_NestedHierarchy(t *testing.T) {
	// Pattern: claude-skills — nested like marketing-skill/copywriting/
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "marketing", "copywriting", "SKILL.md"), "---\nname: copywriting\n---\n")
	writeFile(t, filepath.Join(root, "marketing", "seo", "SKILL.md"), "---\nname: seo\n---\n")
	writeFile(t, filepath.Join(root, "engineering", "api-design", "SKILL.md"), "---\nname: api-design\n---\n")

	dirs, err := ResolveMembers(root, []string{"marketing/*", "engineering/*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("got %d dirs, want 3: %v", len(dirs), dirs)
	}
}

func TestResolveMembers_TieredDotDirs(t *testing.T) {
	// Pattern: codex-skills — skills/.curated/*, skills/.system/*
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skills", ".curated", "figma", "SKILL.md"), "---\nname: figma\n---\n")
	writeFile(t, filepath.Join(root, "skills", ".curated", "gh-pr", "SKILL.md"), "---\nname: gh-pr\n---\n")
	writeFile(t, filepath.Join(root, "skills", ".system", "installer", "SKILL.md"), "---\nname: installer\n---\n")

	dirs, err := ResolveMembers(root, []string{"skills/.curated/*", "skills/.system/*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("got %d dirs, want 3: %v", len(dirs), dirs)
	}
}

func TestResolveMembers_ExcludePatterns(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skill-a", "SKILL.md"), "---\nname: a\n---\n")
	writeFile(t, filepath.Join(root, "docs", "SKILL.md"), "---\nname: docs\n---\n") // should be excluded
	writeFile(t, filepath.Join(root, "scripts", "SKILL.md"), "---\nname: scripts\n---\n")

	dirs, err := ResolveMembers(root, []string{"*"}, []string{"docs", "scripts"})
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("got %d dirs, want 1: %v", len(dirs), dirs)
	}
}

func TestResolveMembers_EmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "empty-dir")) // no SKILL.md

	dirs, err := ResolveMembers(root, []string{"skills/*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("got %d dirs, want 0", len(dirs))
	}
}

func TestResolveMembers_NestedWorkspaceError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "nested", FileName), "name: \"@test/nested\"\ntype: workspace\ndescription: nested\nworkspace:\n  members:\n    - \"*\"\n")

	_, err := ResolveMembers(root, []string{"*"}, nil)
	if err == nil {
		t.Fatal("expected error for nested workspace")
	}
	if want := "nested workspace"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q should contain %q", err.Error(), want)
	}
}

func TestResolveMembers_SkipNonSkillDirs(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "empty-dir"))                     // no SKILL.md, no ctx.yaml
	writeFile(t, filepath.Join(root, "real-skill", "SKILL.md"), "---\nname: real\n---\n")

	dirs, err := ResolveMembers(root, []string{"*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("got %d dirs, want 1", len(dirs))
	}
}

func TestResolveMembers_LargeWorkspace(t *testing.T) {
	// Stress test: 100+ skills (simulate claude-skills scale)
	root := t.TempDir()
	for i := 0; i < 120; i++ {
		name := filepath.Join(root, "skills", fmt.Sprintf("skill-%03d", i))
		writeFile(t, filepath.Join(name, "SKILL.md"), fmt.Sprintf("---\nname: skill-%03d\ndescription: s\n---\n", i))
	}

	dirs, err := ResolveMembers(root, []string{"skills/*"}, nil)
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	if len(dirs) != 120 {
		t.Fatalf("got %d dirs, want 120", len(dirs))
	}
}

// --- ApplyDefaults tests ---

func TestApplyDefaults_ScopeInherited(t *testing.T) {
	m := &Manifest{Name: "translate", Version: "1.0.0", Type: TypeSkill, Description: "t"}
	ApplyDefaults(m, &WorkspaceDefaults{Scope: "@baoyu"})
	if m.Name != "@baoyu/translate" {
		t.Errorf("Name = %q, want @baoyu/translate", m.Name)
	}
}

func TestApplyDefaults_ScopeStripsPrefix(t *testing.T) {
	// baoyu-skills pattern: skill name "baoyu-translate" + scope "@baoyu" → "@baoyu/translate"
	m := &Manifest{Name: "baoyu-translate", Version: "1.0.0", Type: TypeSkill, Description: "t"}
	ApplyDefaults(m, &WorkspaceDefaults{Scope: "@baoyu"})
	if m.Name != "@baoyu/translate" {
		t.Errorf("Name = %q, want @baoyu/translate (scope prefix stripped)", m.Name)
	}
}

func TestApplyDefaults_ChildOverridesParent(t *testing.T) {
	m := &Manifest{
		Name:    "@custom/translate",
		Version: "1.0.0",
		Type:    TypeSkill,
		Description: "t",
		Author:  "Custom Author",
		License: "Apache-2.0",
	}
	ApplyDefaults(m, &WorkspaceDefaults{
		Scope:  "@baoyu",
		Author: "Default Author",
		License: "MIT",
	})
	// Child already has scope, author, and license — should not be overwritten.
	if m.Name != "@custom/translate" {
		t.Errorf("Name = %q, want @custom/translate (child override)", m.Name)
	}
	if m.Author != "Custom Author" {
		t.Errorf("Author = %q, want Custom Author (child override)", m.Author)
	}
	if m.License != "Apache-2.0" {
		t.Errorf("License = %q, want Apache-2.0 (child override)", m.License)
	}
}

func TestApplyDefaults_NoDefaultsSet(t *testing.T) {
	m := &Manifest{Name: "translate", Version: "1.0.0", Type: TypeSkill, Description: "t"}
	ApplyDefaults(m, &WorkspaceDefaults{})
	if m.Name != "translate" {
		t.Errorf("Name = %q, want translate (no scope default)", m.Name)
	}
}

func TestApplyDefaults_AuthorAndLicense(t *testing.T) {
	m := &Manifest{Name: "@test/s", Version: "1.0.0", Type: TypeSkill, Description: "t"}
	ApplyDefaults(m, &WorkspaceDefaults{
		Author:     "Workspace Author",
		License:    "MIT",
		Repository: "https://github.com/test/repo",
	})
	if m.Author != "Workspace Author" {
		t.Errorf("Author = %q, want Workspace Author", m.Author)
	}
	if m.License != "MIT" {
		t.Errorf("License = %q, want MIT", m.License)
	}
	if m.Repository != "https://github.com/test/repo" {
		t.Errorf("Repository = %q, want https://github.com/test/repo", m.Repository)
	}
}

// --- Auto-scaffold tests ---

func TestAutoScaffold_MinimalFrontmatter(t *testing.T) {
	// Pattern: anthropic/skills — only name + description
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "SKILL.md"), "---\nname: pdf\ndescription: PDF processing\n---\n# PDF\n")

	m, err := ScaffoldFromSkillMD(root)
	if err != nil {
		t.Fatalf("ScaffoldFromSkillMD: %v", err)
	}
	if m.Name != "pdf" {
		t.Errorf("Name = %q, want pdf", m.Name)
	}
	if m.Description != "PDF processing" {
		t.Errorf("Description = %q, want 'PDF processing'", m.Description)
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, want 0.1.0", m.Version)
	}
	if m.Skill == nil || m.Skill.Entry != "SKILL.md" {
		t.Errorf("Skill.Entry = %v, want SKILL.md", m.Skill)
	}
}

func TestAutoScaffold_NoFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "SKILL.md"), "# Just content\nNo frontmatter here.\n")

	_, err := ScaffoldFromSkillMD(root)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestAutoScaffold_RichFrontmatter(t *testing.T) {
	// Pattern: baoyu-skills — name + description + version + license
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "SKILL.md"), `---
name: baoyu-translate
description: "Multi-mode translation (quick/normal/refined) with glossary support"
version: "1.5.0"
---
# Translation Skill
`)

	m, err := ScaffoldFromSkillMD(root)
	if err != nil {
		t.Fatalf("ScaffoldFromSkillMD: %v", err)
	}
	if m.Name != "baoyu-translate" {
		t.Errorf("Name = %q, want baoyu-translate", m.Name)
	}
	if m.Description != "Multi-mode translation (quick/normal/refined) with glossary support" {
		t.Errorf("Description = %q", m.Description)
	}
	// Version from scaffold is always 0.1.0 (SKILL.md version is not used for ctx.yaml)
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, want 0.1.0", m.Version)
	}
	if m.Type != TypeSkill {
		t.Errorf("Type = %q, want skill", m.Type)
	}
}

func TestAutoScaffold_OpenClawMetadata(t *testing.T) {
	// Pattern: clawhub — SKILL.md with metadata.openclaw
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "SKILL.md"), `---
name: peekaboo
description: "Capture and automate macOS UI with Peekaboo CLI"
metadata:
  openclaw:
    requires:
      env: [PADEL_AUTH_FILE]
      bins: [padel]
    install:
      brew: [padel]
---
# Peekaboo
`)

	// Standard scaffold still works (OpenClaw metadata is ignored by basic scaffold)
	m, err := ScaffoldFromSkillMD(root)
	if err != nil {
		t.Fatalf("ScaffoldFromSkillMD: %v", err)
	}
	if m.Name != "peekaboo" {
		t.Errorf("Name = %q, want peekaboo", m.Name)
	}

	// But OpenClaw-aware parsing extracts the metadata
	f, _ := os.Open(filepath.Join(root, "SKILL.md"))
	defer f.Close()
	fm, err := ParseOpenClawFrontmatter(f)
	if err != nil {
		t.Fatalf("ParseOpenClawFrontmatter: %v", err)
	}
	oc := fm.GetOpenClawMetadata()
	if oc == nil {
		t.Fatal("expected non-nil OpenClaw metadata")
	}

	ctxM := OpenClawToCtx(fm)
	if ctxM.Install == nil || ctxM.Install.Brew != "padel" {
		t.Errorf("OpenClawToCtx should extract install.brew=padel, got %v", ctxM.Install)
	}
}

func TestLoadWorkspace_SymlinkSkill(t *testing.T) {
	// Pattern: last30days — SKILL.md is a symlink
	root := t.TempDir()

	// Create a real skill file
	realSkillDir := filepath.Join(root, "real")
	writeFile(t, filepath.Join(realSkillDir, "SKILL.md"), "---\nname: linked-skill\ndescription: Symlinked\n---\n")

	// Create a symlink dir pointing to it
	symlinkDir := filepath.Join(root, "skills", "linked")
	mkdirAll(t, filepath.Join(root, "skills"))
	if err := os.Symlink(realSkillDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	writeFile(t, filepath.Join(root, FileName),
		"name: \"@test/skills\"\ntype: workspace\ndescription: Test\nworkspace:\n  members:\n    - \"skills/*\"\n  defaults:\n    scope: \"@test\"\n")

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Members) != 1 {
		t.Fatalf("got %d members, want 1", len(ws.Members))
	}
	if ws.Members[0].Manifest.Name != "@test/linked-skill" {
		t.Errorf("Name = %q, want @test/linked-skill", ws.Members[0].Manifest.Name)
	}
}

func TestAutoScaffold_NoName_FallsBackToDir(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-cool-skill")
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "---\ndescription: Some skill\n---\n")

	m, err := ScaffoldFromSkillMD(skillDir)
	if err != nil {
		t.Fatalf("ScaffoldFromSkillMD: %v", err)
	}
	if m.Name != "my-cool-skill" {
		t.Errorf("Name = %q, want my-cool-skill (dir fallback)", m.Name)
	}
}

// --- LoadWorkspace tests ---

func TestLoadWorkspace_FlatPattern(t *testing.T) {
	root := t.TempDir()
	setupWorkspace(t, root,
		"name: \"@test/skills\"\ntype: workspace\ndescription: Test workspace\nworkspace:\n  members:\n    - \"skills/*\"\n  defaults:\n    scope: \"@test\"\n",
		map[string]string{
			"skills/alpha": "---\nname: alpha\ndescription: Alpha skill\n---\n",
			"skills/beta":  "---\nname: beta\ndescription: Beta skill\n---\n",
		},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(ws.Members))
	}

	// Check defaults were applied (scope).
	for _, m := range ws.Members {
		scope, _ := ParseFullName(m.Manifest.Name)
		if scope != "test" {
			t.Errorf("member %q should have scope @test", m.Manifest.Name)
		}
	}
}

func TestLoadWorkspace_WithCtxYaml(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName),
		"name: \"@test/skills\"\ntype: workspace\ndescription: Test\nworkspace:\n  members:\n    - \"skills/*\"\n  defaults:\n    scope: \"@test\"\n")
	// Member with its own ctx.yaml (SSOT).
	writeFile(t, filepath.Join(root, "skills", "translate", FileName),
		"name: \"@custom/translate\"\nversion: \"2.0.0\"\ntype: skill\ndescription: Custom\nskill:\n  entry: SKILL.md\n")
	writeFile(t, filepath.Join(root, "skills", "translate", "SKILL.md"), "---\nname: translate\n---\n")

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Members) != 1 {
		t.Fatalf("got %d members, want 1", len(ws.Members))
	}

	m := ws.Members[0]
	if m.Source != "ctx.yaml" {
		t.Errorf("Source = %q, want ctx.yaml", m.Source)
	}
	// ctx.yaml has explicit scope, so defaults should NOT override.
	if m.Manifest.Name != "@custom/translate" {
		t.Errorf("Name = %q, want @custom/translate (ctx.yaml is SSOT)", m.Manifest.Name)
	}
	if m.Manifest.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", m.Manifest.Version)
	}
}

func TestLoadWorkspace_DuplicateNames(t *testing.T) {
	root := t.TempDir()
	setupWorkspace(t, root,
		"name: \"@test/skills\"\ntype: workspace\ndescription: Test\nworkspace:\n  members:\n    - \"skills/*\"\n",
		map[string]string{
			"skills/alpha": "---\nname: dupe\ndescription: First\n---\n",
			"skills/beta":  "---\nname: dupe\ndescription: Second\n---\n",
		},
	)

	_, err := LoadWorkspace(root)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error %q should mention 'duplicate'", err.Error())
	}
}

func TestLoadWorkspace_NotWorkspaceType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName),
		"name: \"@test/skill\"\nversion: \"1.0.0\"\ntype: skill\ndescription: Not a workspace\nskill:\n  entry: SKILL.md\n")

	_, err := LoadWorkspace(root)
	if err == nil {
		t.Fatal("expected error for non-workspace type")
	}
}

// --- ResolveCollections tests ---

func TestResolveCollections_MultipleGroups(t *testing.T) {
	// Pattern: anthropic/skills — 3 plugin groups
	root := t.TempDir()
	setupWorkspace(t, root,
		`name: "@test/skills"
type: workspace
description: Test
workspace:
  members:
    - "skills/*"
  defaults:
    scope: "@test"
  collections:
    - name: document-skills
      description: Document processing
      members: [xlsx, docx, pdf]
    - name: creative-skills
      description: Creative tools
      members: [art, music]
`,
		map[string]string{
			"skills/xlsx":  "---\nname: xlsx\ndescription: Excel\n---\n",
			"skills/docx":  "---\nname: docx\ndescription: Word\n---\n",
			"skills/pdf":   "---\nname: pdf\ndescription: PDF\n---\n",
			"skills/art":   "---\nname: art\ndescription: Art\n---\n",
			"skills/music": "---\nname: music\ndescription: Music\n---\n",
		},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	collections, err := ResolveCollections(ws)
	if err != nil {
		t.Fatalf("ResolveCollections: %v", err)
	}

	if len(collections) != 2 {
		t.Fatalf("got %d collections, want 2", len(collections))
	}
	if len(collections["document-skills"]) != 3 {
		t.Errorf("document-skills has %d members, want 3", len(collections["document-skills"]))
	}
	if len(collections["creative-skills"]) != 2 {
		t.Errorf("creative-skills has %d members, want 2", len(collections["creative-skills"]))
	}
}

func TestResolveCollections_GlobMembers(t *testing.T) {
	// Pattern: claude-skills — collection with glob pattern matching relative dirs
	root := t.TempDir()
	setupWorkspace(t, root,
		`name: "@test/skills"
type: workspace
description: Test
workspace:
  members:
    - "marketing/*"
    - "engineering/*"
  defaults:
    scope: "@test"
  collections:
    - name: marketing-skills
      description: Marketing
      members: ["marketing/*"]
`,
		map[string]string{
			"marketing/copywriting": "---\nname: copywriting\ndescription: Copy\n---\n",
			"marketing/seo":        "---\nname: seo\ndescription: SEO\n---\n",
			"engineering/api":      "---\nname: api\ndescription: API\n---\n",
		},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	collections, err := ResolveCollections(ws)
	if err != nil {
		t.Fatalf("ResolveCollections: %v", err)
	}

	if len(collections["marketing-skills"]) != 2 {
		t.Errorf("marketing-skills has %d members, want 2", len(collections["marketing-skills"]))
	}
}

func TestResolveCollections_MemberNotFound(t *testing.T) {
	root := t.TempDir()
	setupWorkspace(t, root,
		`name: "@test/skills"
type: workspace
description: Test
workspace:
  members:
    - "skills/*"
  collections:
    - name: missing-collection
      description: Has missing member
      members: [nonexistent]
`,
		map[string]string{
			"skills/alpha": "---\nname: alpha\ndescription: A\n---\n",
		},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	_, err = ResolveCollections(ws)
	if err == nil {
		t.Fatal("expected error for collection with no matching members")
	}
}

func TestResolveCollections_NoCollections(t *testing.T) {
	root := t.TempDir()
	setupWorkspace(t, root,
		"name: \"@test/skills\"\ntype: workspace\ndescription: Test\nworkspace:\n  members:\n    - \"skills/*\"\n",
		map[string]string{
			"skills/alpha": "---\nname: alpha\ndescription: A\n---\n",
		},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	collections, err := ResolveCollections(ws)
	if err != nil {
		t.Fatalf("ResolveCollections: %v", err)
	}
	if collections != nil {
		t.Errorf("expected nil collections for workspace without collections")
	}
}

