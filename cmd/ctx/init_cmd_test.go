package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/prompt"
)

// --- detectInitInput tests ---

func TestDetectInitInput_NoArgs(t *testing.T) {
	input, err := detectInitInput(nil)
	if err != nil {
		t.Fatal(err)
	}
	if input.mode != initFromScratch {
		t.Errorf("mode = %d, want initFromScratch", input.mode)
	}
}

func TestDetectInitInput_MDFile(t *testing.T) {
	tmp := t.TempDir()
	mdFile := filepath.Join(tmp, "gc.md")
	if err := os.WriteFile(mdFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, err := detectInitInput([]string{mdFile})
	if err != nil {
		t.Fatal(err)
	}
	if input.mode != initFromFile {
		t.Errorf("mode = %d, want initFromFile", input.mode)
	}
	if input.sourcePath != mdFile {
		t.Errorf("sourcePath = %q, want %q", input.sourcePath, mdFile)
	}
}

func TestDetectInitInput_Directory(t *testing.T) {
	tmp := t.TempDir()

	input, err := detectInitInput([]string{tmp})
	if err != nil {
		t.Fatal(err)
	}
	if input.mode != initFromDirectory {
		t.Errorf("mode = %d, want initFromDirectory", input.mode)
	}
}

func TestDetectInitInput_NonMDFile(t *testing.T) {
	tmp := t.TempDir()
	txtFile := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(txtFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := detectInitInput([]string{txtFile})
	if err == nil {
		t.Fatal("expected error for non-.md file")
	}
}

func TestDetectInitInput_FileNotFound(t *testing.T) {
	_, err := detectInitInput([]string{"/nonexistent/gc.md"})
	if err == nil {
		t.Fatal("expected error for missing .md file")
	}
}

func TestDetectInitInput_NonExistentPath(t *testing.T) {
	_, err := detectInitInput([]string{"/nonexistent/my-skill"})
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "path not found") {
		t.Errorf("error = %q, want 'path not found'", err)
	}
}

// --- parseInitSource tests ---

func TestParseFileSource_WithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	content := `---
name: gc
description: Generate commit messages
triggers:
  - /gc
  - git commit
invocable: true
argument-hint: "<message>"
---

# GC Skill

This skill generates commit messages.
`
	mdFile := filepath.Join(tmp, "gc.md")
	if err := os.WriteFile(mdFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseFileSource(mdFile)
	if err != nil {
		t.Fatal(err)
	}

	if meta.name != "gc" {
		t.Errorf("name = %q, want %q", meta.name, "gc")
	}
	if meta.description != "Generate commit messages" {
		t.Errorf("description = %q", meta.description)
	}
	if len(meta.triggers) != 2 {
		t.Errorf("triggers len = %d, want 2", len(meta.triggers))
	}
	if !meta.invocable {
		t.Error("invocable should be true")
	}
	if meta.argHint != "<message>" {
		t.Errorf("argHint = %q", meta.argHint)
	}
	if !strings.Contains(meta.body, "GC Skill") {
		t.Errorf("body missing content: %q", meta.body)
	}
}

func TestParseFileSource_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	content := "# My Skill\n\nThis is a simple skill for testing.\n"
	mdFile := filepath.Join(tmp, "my-tool.md")
	if err := os.WriteFile(mdFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseFileSource(mdFile)
	if err != nil {
		t.Fatal(err)
	}

	if meta.name != "my-tool" {
		t.Errorf("name = %q, want %q", meta.name, "my-tool")
	}
	if meta.description != "This is a simple skill for testing." {
		t.Errorf("description = %q", meta.description)
	}
	if len(meta.triggers) == 0 || meta.triggers[0] != "/my-tool" {
		t.Errorf("triggers = %v, want [/my-tool]", meta.triggers)
	}
}

func TestParseFileSource_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	mdFile := filepath.Join(tmp, "empty.md")
	if err := os.WriteFile(mdFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := parseFileSource(mdFile)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestParseDirSource_WithCtxYaml(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "my-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := manifest.Scaffold(manifest.TypeSkill, "testuser", "my-skill")
	m.Version = "1.2.0"
	m.Description = "A test skill"
	m.Keywords = []string{"/my-skill", "test"}
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseDirSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	if meta.name != "my-skill" {
		t.Errorf("name = %q, want %q", meta.name, "my-skill")
	}
	if meta.version != "1.2.0" {
		t.Errorf("version = %q, want %q", meta.version, "1.2.0")
	}
	if meta.description != "A test skill" {
		t.Errorf("description = %q", meta.description)
	}
	if len(meta.triggers) != 2 {
		t.Errorf("triggers len = %d, want 2", len(meta.triggers))
	}
}

func TestParseDirSource_WithSKILLMD(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "cool-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: cool
description: A cool skill
triggers:
  - /cool
invocable: true
---

# Cool Skill

Does cool things.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseDirSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	if meta.name != "cool" {
		t.Errorf("name = %q, want %q", meta.name, "cool")
	}
	if meta.description != "A cool skill" {
		t.Errorf("description = %q", meta.description)
	}
}

func TestParseDirSource_Empty(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "empty-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta, err := parseDirSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	if meta.name != "empty-dir" {
		t.Errorf("name = %q, want %q", meta.name, "empty-dir")
	}
	if meta.version != "0.1.0" {
		t.Errorf("version = %q", meta.version)
	}
}

func TestParseDirSource_PreservesArgumentHint(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "my-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write ctx.yaml (no argument-hint field)
	m := manifest.Scaffold(manifest.TypeSkill, "testuser", "my-skill")
	m.Version = "1.0.0"
	m.Description = "A skill"
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write SKILL.md with argument-hint in frontmatter
	skillContent := `---
name: my-skill
description: A skill
argument-hint: "<commit message>"
invocable: true
---

# My Skill
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseDirSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	if meta.argHint != "<commit message>" {
		t.Errorf("argHint = %q, want %q", meta.argHint, "<commit message>")
	}
}

func TestResolveOutputDir_Scratch(t *testing.T) {
	cwd, _ := os.Getwd()
	dir, err := resolveOutputDir(initInput{mode: initFromScratch})
	if err != nil {
		t.Fatal(err)
	}
	if dir != cwd {
		t.Errorf("scratch mode: got %q, want cwd %q", dir, cwd)
	}
}

func TestResolveOutputDir_File(t *testing.T) {
	tmp := t.TempDir()
	mdFile := filepath.Join(tmp, "gc.md")
	dir, err := resolveOutputDir(initInput{mode: initFromFile, sourcePath: mdFile})
	if err != nil {
		t.Fatal(err)
	}
	if dir != tmp {
		t.Errorf("file mode: got %q, want %q", dir, tmp)
	}
}

func TestResolveOutputDir_Directory(t *testing.T) {
	tmp := t.TempDir()
	dir, err := resolveOutputDir(initInput{mode: initFromDirectory, sourceDir: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if dir != tmp {
		t.Errorf("directory mode: got %q, want %q", dir, tmp)
	}
}

// --- promptMetadata tests ---

func TestPromptMetadata_NoopPrompter(t *testing.T) {
	p := prompt.NoopPrompter{}
	meta := initMeta{
		name:        "gc",
		description: "Generate commits",
		version:     "0.1.0",
	}

	result, err := promptMetadata(p, meta)
	if err != nil {
		t.Fatal(err)
	}

	if result.name != "gc" {
		t.Errorf("name = %q, want %q", result.name, "gc")
	}
	if result.description != "Generate commits" {
		t.Errorf("description = %q", result.description)
	}
	if result.version != "0.1.0" {
		t.Errorf("version = %q", result.version)
	}
}

// --- resolveScope tests ---

func TestResolveScope_NoConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	scope := resolveScope()
	if scope != "local" {
		t.Errorf("scope = %q, want %q", scope, "local")
	}
}

// --- Full pipeline tests ---

func TestInitPipeline_FromFile_WritesLocally(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create source .md file
	content := `---
name: gc
description: Generate bilingual commit messages
triggers:
  - /gc
invocable: true
---

# GC

Generate commit messages in both Chinese and English.
`
	srcFile := filepath.Join(tmp, "gc.md")
	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Detect
	input, err := detectInitInput([]string{srcFile})
	if err != nil {
		t.Fatal(err)
	}
	if input.mode != initFromFile {
		t.Fatalf("mode = %d, want initFromFile", input.mode)
	}

	// Resolve output directory — should be the file's parent
	outDir, err := resolveOutputDir(input)
	if err != nil {
		t.Fatal(err)
	}
	if outDir != tmp {
		t.Fatalf("outDir = %q, want %q", outDir, tmp)
	}

	// Parse
	meta, err := parseInitSource(input)
	if err != nil {
		t.Fatal(err)
	}

	// Prompt (noop)
	meta, err = promptMetadata(prompt.NoopPrompter{}, meta)
	if err != nil {
		t.Fatal(err)
	}

	// Build manifest — skill type from file defaults to skill
	scope := "local"
	skillName := slugify(meta.name)
	fullName := manifest.FormatFullName(scope, skillName)

	m := manifest.Scaffold(manifest.TypeSkill, scope, skillName)
	m.Version = meta.version
	m.Description = meta.description
	m.Keywords = meta.triggers
	m.Skill.UserInvocable = &meta.invocable

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Fatalf("validation errors: %v", errs)
	}

	manifestData, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	// Write ctx.yaml to the file's directory (local write, no staging)
	if err := os.WriteFile(filepath.Join(outDir, "ctx.yaml"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Generate SKILL.md (since none exists at entry path)
	skillEntry := m.Skill.Entry
	skillAbsPath := filepath.Join(outDir, skillEntry)
	fm := &manifest.SkillFrontmatter{
		Name:        skillName,
		Description: meta.description,
		Triggers:    meta.triggers,
		Invocable:   meta.invocable,
	}
	skillContent, err := manifest.RenderSkillMD(fm, meta.body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillAbsPath, skillContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify files are in the local directory, not ~/.ctx/skills/
	loaded, err := manifest.LoadFromDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != fullName {
		t.Errorf("loaded name = %q, want %q", loaded.Name, fullName)
	}
	if loaded.Version != "0.1.0" {
		t.Errorf("version = %q", loaded.Version)
	}

	// Verify SKILL.md roundtrips
	f, err := os.Open(skillAbsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	parsedFM, parsedBody, err := manifest.ParseSkillMD(f)
	if err != nil {
		t.Fatal(err)
	}
	if parsedFM == nil || parsedFM.Name != "gc" {
		t.Errorf("parsed name = %v", parsedFM)
	}
	if !strings.Contains(parsedBody, "Generate commit messages") {
		t.Errorf("body missing content: %q", parsedBody)
	}
}

func TestInitPipeline_LinkBack(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create source and target
	srcFile := filepath.Join(tmp, "gc.md")
	if err := os.WriteFile(srcFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmp, "skills", "local", "gc")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(destDir, "SKILL.md")
	if err := os.WriteFile(targetFile, []byte("skill content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Link
	if err := linkToOriginal(srcFile, targetFile, "@local/gc"); err != nil {
		t.Fatal(err)
	}

	// Verify symlink
	link, err := os.Readlink(srcFile)
	if err != nil {
		t.Fatalf("should be a symlink: %v", err)
	}
	if link != targetFile {
		t.Errorf("link = %q, want %q", link, targetFile)
	}

	// Verify backup
	bakData, err := os.ReadFile(srcFile + ".bak")
	if err != nil {
		t.Fatalf("backup should exist: %v", err)
	}
	if string(bakData) != "original" {
		t.Errorf("backup content = %q", string(bakData))
	}
}


func TestInitPipeline_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// First write — ctx.yaml in project directory
	m := manifest.Scaffold(manifest.TypeSkill, "local", "gc")
	m.Version = "0.1.0"
	m.Description = "test"
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify it exists
	if _, err := os.Stat(filepath.Join(dir, "ctx.yaml")); err != nil {
		t.Fatal("first write should exist")
	}

	// Second write (overwrite) — simulates running init twice
	m.Version = "0.2.0"
	data2, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data2, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify updated
	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != "0.2.0" {
		t.Errorf("version = %q, want %q", loaded.Version, "0.2.0")
	}
}

func TestInitPipeline_CLIWritesNestedSkill(t *testing.T) {
	dir := t.TempDir()

	// Scaffold a CLI manifest — Scaffold now includes skill section
	m := manifest.Scaffold(manifest.TypeCLI, "local", "fizzy")
	m.Version = "0.1.0"
	m.Description = "Fizzy CLI"
	m.CLI.Binary = "fizzy"
	m.CLI.Verify = "fizzy --version"
	m.Install = &manifest.InstallSpec{Script: "https://example.com/install.sh"}
	m.Skill.Origin = "native"

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Fatalf("validation errors: %v", errs)
	}

	manifestData, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	// Write ctx.yaml locally
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write SKILL.md at the nested entry path
	skillEntry := m.Skill.Entry // "skills/fizzy/SKILL.md"
	skillAbsPath := filepath.Join(dir, skillEntry)
	if err := os.MkdirAll(filepath.Dir(skillAbsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	fm := &manifest.SkillFrontmatter{
		Name:        "fizzy",
		Description: "Fizzy CLI",
		Triggers:    []string{"/fizzy", "fizzy"},
		Invocable:   true,
	}
	skillContent, err := manifest.RenderSkillMD(fm, "# Fizzy\n\nA fizzy CLI tool.\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillAbsPath, skillContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify ctx.yaml
	loaded, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "@local/fizzy" {
		t.Errorf("name = %q, want @local/fizzy", loaded.Name)
	}
	if loaded.Skill == nil || loaded.Skill.Entry != "skills/fizzy/SKILL.md" {
		t.Errorf("skill.entry = %v, want skills/fizzy/SKILL.md", loaded.Skill)
	}

	// Verify SKILL.md exists at nested path
	if _, err := os.Stat(skillAbsPath); err != nil {
		t.Errorf("SKILL.md should exist at %s", skillEntry)
	}
}

func TestScaffold_AllTypesHaveSkill(t *testing.T) {
	for _, tt := range []struct {
		pkgType       manifest.PackageType
		expectedEntry string
		skillOptional bool
	}{
		{manifest.TypeSkill, "SKILL.md", false},
		{manifest.TypeCLI, "skills/test-pkg/SKILL.md", false},
		{manifest.TypeMCP, "", true}, // MCP: skill is optional
	} {
		m := manifest.Scaffold(tt.pkgType, "local", "test-pkg")
		if tt.skillOptional {
			// MCP: Skill may be nil, that's OK
			errs := manifest.Validate(m)
			if len(errs) > 0 {
				t.Errorf("%s: validation errors: %v", tt.pkgType, errs)
			}
		} else {
			if m.Skill == nil {
				t.Errorf("%s: Skill should not be nil", tt.pkgType)
				continue
			}
			if m.Skill.Entry != tt.expectedEntry {
				t.Errorf("%s: Skill.Entry = %q, want %q", tt.pkgType, m.Skill.Entry, tt.expectedEntry)
			}
			errs := manifest.Validate(m)
			if len(errs) > 0 {
				t.Errorf("%s: validation errors: %v", tt.pkgType, errs)
			}
		}
	}
}

// --- initCmd registration tests ---

func TestInitCmd_AcceptsArgs(t *testing.T) {
	// init should accept 0 or 1 args
	if err := initCmd.Args(initCmd, nil); err != nil {
		t.Errorf("should accept 0 args: %v", err)
	}
	if err := initCmd.Args(initCmd, []string{"gc.md"}); err != nil {
		t.Errorf("should accept 1 arg: %v", err)
	}
	if err := initCmd.Args(initCmd, []string{"a", "b"}); err == nil {
		t.Error("should reject 2 args")
	}
}

// --- findAllSkillMD tests ---

func TestFindAllSkillMD_RootOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# root"), 0o644); err != nil {
		t.Fatal(err)
	}

	found := findAllSkillMD(dir)
	if len(found) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(found), found)
	}
	if found[0] != "SKILL.md" {
		t.Errorf("expected SKILL.md, got %q", found[0])
	}
}

func TestFindAllSkillMD_SubdirOnly(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "fizzy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# fizzy"), 0o644); err != nil {
		t.Fatal(err)
	}

	found := findAllSkillMD(dir)
	if len(found) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(found), found)
	}
	expected := filepath.Join("skills", "fizzy", "SKILL.md")
	if found[0] != expected {
		t.Errorf("expected %q, got %q", expected, found[0])
	}
}

func TestFindAllSkillMD_Multiple(t *testing.T) {
	dir := t.TempDir()
	// Root
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# root"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Subdir
	skillDir := filepath.Join(dir, "skills", "foo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# foo"), 0o644); err != nil {
		t.Fatal(err)
	}

	found := findAllSkillMD(dir)
	if len(found) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(found), found)
	}
	// Root should be first
	if found[0] != "SKILL.md" {
		t.Errorf("first result should be root SKILL.md, got %q", found[0])
	}
}

func TestFindAllSkillMD_None(t *testing.T) {
	dir := t.TempDir()

	found := findAllSkillMD(dir)
	if len(found) != 0 {
		t.Errorf("expected 0 results, got %d: %v", len(found), found)
	}
}
