package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/license"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// importFormat describes which repo layout was detected.
type importFormat int

const (
	importFormatMarketplace  importFormat = iota // .claude-plugin/marketplace.json
	importFormatCodex                            // skills/.curated/ or skills/.system/
	importFormatSingleSkill                      // root SKILL.md with frontmatter
	importFormatFlatSkills                       // */SKILL.md one level deep
	importFormatNestedSkills                     // */*/SKILL.md two levels deep
	importFormatBareMarkdown                     // *.md without frontmatter (non-README)
	importFormatUnknown                          // not a skill repo
)

func (f importFormat) String() string {
	switch f {
	case importFormatMarketplace:
		return "marketplace.json"
	case importFormatCodex:
		return "codex (.curated/.system)"
	case importFormatSingleSkill:
		return "single skill"
	case importFormatFlatSkills:
		return "flat skill directories"
	case importFormatNestedSkills:
		return "nested skill directories"
	case importFormatBareMarkdown:
		return "bare markdown"
	default:
		return "unknown"
	}
}

// importDetection holds the result of scanning a directory.
type importDetection struct {
	format     importFormat
	skills     []importedSkill
	rootDir    string // absolute path
	memberGlobs []string // workspace member patterns (e.g., ["skills/*"])
}

// importedSkill represents a single detected skill.
type importedSkill struct {
	name           string // from frontmatter or directory name
	description    string
	dir            string // relative path from root
	entry          string // relative to skill dir, default "SKILL.md"
	version        string
	tags           []string
	hasFrontmatter bool
}

// detectImportFormat scans a directory and identifies the skill repo format.
func detectImportFormat(dir string) (*importDetection, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	det := &importDetection{rootDir: absDir}

	// Priority 1: marketplace.json
	mpPath := filepath.Join(absDir, ".claude-plugin", "marketplace.json")
	if fileExistsAt(mpPath) {
		skills, globs, err := detectFromMarketplace(absDir, mpPath)
		if err != nil {
			return nil, fmt.Errorf("parse marketplace.json: %w", err)
		}
		det.format = importFormatMarketplace
		det.skills = skills
		det.memberGlobs = globs
		return det, nil
	}

	// Priority 2: Codex format
	curatedDir := filepath.Join(absDir, "skills", ".curated")
	systemDir := filepath.Join(absDir, "skills", ".system")
	if dirExistsAt(curatedDir) || dirExistsAt(systemDir) {
		skills := scanSkillDirs(absDir, "skills/.curated")
		skills = append(skills, scanSkillDirs(absDir, "skills/.system")...)
		det.format = importFormatCodex
		det.skills = skills
		// Include both .curated and .system in member globs
		var memberGlobs []string
		if dirExistsAt(curatedDir) {
			memberGlobs = append(memberGlobs, "skills/.curated/*")
		}
		if dirExistsAt(systemDir) {
			memberGlobs = append(memberGlobs, "skills/.system/*")
		}
		det.memberGlobs = memberGlobs
		return det, nil
	}

	// Priority 3: Root SKILL.md with frontmatter
	rootSkill := filepath.Join(absDir, "SKILL.md")
	if fileExistsAt(rootSkill) {
		skill, hasFM := parseSkillAt(absDir, ".", "SKILL.md")
		if hasFM {
			// For root-level skills, use the repo directory name if name is empty
			if skill.name == "" || skill.name == "." {
				skill.name = slugify(filepath.Base(absDir))
			}
			det.format = importFormatSingleSkill
			det.skills = []importedSkill{skill}
			return det, nil
		}
	}

	// Priority 4: Flat */SKILL.md (one level)
	flatSkills := deduplicateSkills(scanSkillGlob(absDir, "*/SKILL.md"))
	if len(flatSkills) > 0 {
		if len(flatSkills) == 1 {
			// Single unique skill — treat as single-skill, not workspace
			det.format = importFormatSingleSkill
			det.skills = flatSkills
			return det, nil
		}
		det.format = importFormatFlatSkills
		det.skills = flatSkills
		det.memberGlobs = inferMemberGlobs(flatSkills)
		return det, nil
	}

	// Priority 5: Nested */*/SKILL.md (two levels)
	nestedSkills := deduplicateSkills(scanSkillGlob(absDir, "*/*/SKILL.md"))
	if len(nestedSkills) > 0 {
		if len(nestedSkills) == 1 {
			// Single unique skill — treat as single-skill, not workspace
			det.format = importFormatSingleSkill
			det.skills = nestedSkills
			return det, nil
		}
		det.format = importFormatNestedSkills
		det.skills = nestedSkills
		det.memberGlobs = inferMemberGlobs(nestedSkills)
		return det, nil
	}

	// Priority 6: Bare markdown (non-README *.md without frontmatter)
	bareSkills := scanBareMarkdown(absDir)
	if len(bareSkills) > 0 {
		det.format = importFormatBareMarkdown
		det.skills = bareSkills
		return det, nil
	}

	// Priority 7: Unknown
	det.format = importFormatUnknown
	return det, nil
}

// runInitImport is the entry point for `ctx init --import`.
func runInitImport(cmd *cobra.Command, w *output.Writer) error {
	dir := "."
	args := cmd.Flags().Args()
	if len(args) > 0 {
		dir = args[0]
	}

	det, err := detectImportFormat(dir)
	if err != nil {
		return err
	}

	if det.format == importFormatUnknown {
		return output.ErrUsageHint(
			"could not detect a skill repo format in this directory",
			"Ensure the directory contains SKILL.md files or .claude-plugin/marketplace.json",
		)
	}

	w.Info("Detected format: %s (%d skill(s))", det.format, len(det.skills))

	// Resolve scope
	scope := ""
	if flagInitName != "" {
		s, _ := manifest.ParseFullName(flagInitName)
		if s != "" {
			scope = s
		} else {
			scope = strings.TrimPrefix(flagInitName, "@")
		}
	}
	if scope == "" {
		scope = resolveScope()
	}
	if scope == "" || scope == "local" {
		if term.IsTerminal(int(os.Stdin.Fd())) && !flagYes {
			p := prompt.DefaultPrompter()
			scope, err = p.Text("Package scope (e.g., your username)", "local")
			if err != nil {
				return err
			}
		} else {
			scope = "local"
		}
	}
	scope = strings.TrimPrefix(scope, "@")

	// Detect git metadata for workspace defaults
	absDir, _ := filepath.Abs(dir)
	author := gitutil.Author(absDir)
	repo := gitutil.RemoteURL(absDir)
	lic := ""
	if lr := license.Detect(absDir); lr.SPDX != "" {
		lic = lr.SPDX
	}

	var filesWritten int

	switch det.format {
	case importFormatSingleSkill:
		skill := det.skills[0]
		n, writeErr := writeSingleSkillCtxYaml(absDir, skill, scope, author, lic, repo, w)
		if writeErr != nil {
			return writeErr
		}
		filesWritten = n

	case importFormatBareMarkdown:
		skill := det.skills[0]
		if skill.name == "" {
			if term.IsTerminal(int(os.Stdin.Fd())) && !flagYes {
				p := prompt.DefaultPrompter()
				skill.name, err = p.Text("Skill name", slugify(filepath.Base(absDir)))
				if err != nil {
					return err
				}
				skill.description, err = p.Text("Description", "")
				if err != nil {
					return err
				}
			} else {
				skill.name = slugify(filepath.Base(absDir))
			}
		}
		n, writeErr := writeSingleSkillCtxYaml(absDir, skill, scope, author, lic, repo, w)
		if writeErr != nil {
			return writeErr
		}
		filesWritten = n

	default:
		// Multi-skill: write workspace root + per-skill ctx.yaml
		n, writeErr := writeWorkspaceImport(absDir, det, scope, author, lic, repo, w)
		if writeErr != nil {
			return writeErr
		}
		filesWritten = n
	}

	if filesWritten == 0 {
		w.Info("No files written (all ctx.yaml files already exist)")
		return nil
	}

	w.Info("")
	w.Success("Imported %d skill(s) as @%s — wrote %d file(s)", len(det.skills), scope, filesWritten)
	w.PrintDim("")
	w.PrintDim("  Next steps:")
	w.PrintDim("    1. Review the generated ctx.yaml files")
	if len(det.skills) > 1 {
		w.PrintDim("    2. ctx publish --all --yes     Publish all skills")
	} else {
		w.PrintDim("    2. ctx publish --yes           Publish the skill")
	}
	w.PrintDim("    3. Add .github/workflows/ctx-publish.yml for CI/CD")

	return nil
}

// writeSingleSkillCtxYaml writes a single ctx.yaml for a standalone skill repo.
func writeSingleSkillCtxYaml(rootDir string, skill importedSkill, scope, author, lic, repo string, w *output.Writer) (int, error) {
	ctxPath := filepath.Join(rootDir, manifest.FileName)
	if fileExistsAt(ctxPath) {
		w.Warn("Skipping %s (ctx.yaml already exists)", manifest.FileName)
		return 0, nil
	}

	m := buildSkillManifest(skill, scope)
	m.Author = author
	m.License = lic
	m.Repository = repo

	return writeManifest(ctxPath, m, w)
}

// writeWorkspaceImport writes workspace root ctx.yaml + per-skill ctx.yaml files,
// plus release-please config files.
func writeWorkspaceImport(rootDir string, det *importDetection, scope, author, lic, repo string, w *output.Writer) (int, error) {
	var total int

	// 1. Write workspace root ctx.yaml
	rootCtxPath := filepath.Join(rootDir, manifest.FileName)
	if fileExistsAt(rootCtxPath) {
		w.Warn("Skipping root ctx.yaml (already exists)")
	} else {
		wsName := slugify(filepath.Base(rootDir))
		desc := fmt.Sprintf("Workspace for @%s skills", scope)
		m := &manifest.Manifest{
			Name:        manifest.FormatFullName(scope, wsName),
			Type:        manifest.TypeWorkspace,
			Description: desc,
			Workspace: &manifest.WorkspaceSpec{
				Members: det.memberGlobs,
				Defaults: &manifest.WorkspaceDefaults{
					Scope:      scope,
					Author:     author,
					License:    lic,
					Repository: repo,
				},
			},
		}
		n, err := writeManifest(rootCtxPath, m, w)
		if err != nil {
			return total, err
		}
		total += n
	}

	// 2. Write per-skill ctx.yaml
	for _, skill := range det.skills {
		skillCtxPath := filepath.Join(rootDir, skill.dir, manifest.FileName)
		if fileExistsAt(skillCtxPath) {
			w.PrintDim("  Skipping %s (ctx.yaml already exists)", skill.dir)
			continue
		}

		m := buildSkillManifest(skill, "")
		// Scope will be applied by workspace defaults at load time.
		// Set the bare name so ApplyDefaults can prepend scope.
		m.Name = skill.name

		n, err := writeManifest(skillCtxPath, m, w)
		if err != nil {
			return total, fmt.Errorf("write %s/ctx.yaml: %w", skill.dir, err)
		}
		total += n
	}

	// 3. Write release-please config
	rpWritten, rpErr := writeReleasePleaseConfig(rootDir, det, w)
	if rpErr != nil {
		w.Warn("Could not generate release-please config: %v", rpErr)
	} else {
		total += rpWritten
	}

	return total, nil
}

// buildSkillManifest creates a minimal Manifest for a single skill.
func buildSkillManifest(skill importedSkill, scope string) *manifest.Manifest {
	name := skill.name
	if scope != "" {
		name = manifest.FormatFullName(scope, skill.name)
	}
	version := skill.version
	if version == "" {
		version = "0.1.0"
	}
	entry := skill.entry
	if entry == "" {
		entry = "SKILL.md"
	}

	m := &manifest.Manifest{
		Name:        name,
		Version:     version,
		Type:        manifest.TypeSkill,
		Description: skill.description,
		Skill:       &manifest.SkillSpec{Entry: entry},
	}
	if len(skill.tags) > 0 {
		m.Keywords = skill.tags
	}
	return m
}

// writeReleasePleaseConfig generates release-please-config.json and
// .release-please-manifest.json for monorepo versioning.
func writeReleasePleaseConfig(rootDir string, det *importDetection, w *output.Writer) (int, error) {
	var written int

	// Config
	configPath := filepath.Join(rootDir, "release-please-config.json")
	if !fileExistsAt(configPath) {
		packages := make(map[string]interface{})
		for _, skill := range det.skills {
			packages[skill.dir] = map[string]interface{}{
				"release-type": "simple",
				"changelog-sections": []map[string]interface{}{
					{"type": "feat", "section": "Features"},
					{"type": "fix", "section": "Bug Fixes"},
					{"type": "perf", "section": "Performance"},
					{"type": "docs", "section": "Documentation", "hidden": true},
					{"type": "chore", "section": "Miscellaneous", "hidden": true},
				},
			}
		}
		config := map[string]interface{}{
			"$schema":  "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
			"packages": packages,
		}
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return written, err
		}
		if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
			return written, err
		}
		w.PrintDim("  Wrote release-please-config.json")
		written++
	}

	// Manifest
	manifestPath := filepath.Join(rootDir, ".release-please-manifest.json")
	if !fileExistsAt(manifestPath) {
		versions := make(map[string]string)
		for _, skill := range det.skills {
			v := skill.version
			if v == "" {
				v = "0.1.0"
			}
			versions[skill.dir] = v
		}
		data, err := json.MarshalIndent(versions, "", "  ")
		if err != nil {
			return written, err
		}
		if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
			return written, err
		}
		w.PrintDim("  Wrote .release-please-manifest.json")
		written++
	}

	return written, nil
}

// --- detection helpers ---

func detectFromMarketplace(rootDir, mpPath string) ([]importedSkill, []string, error) {
	f, err := os.Open(mpPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	mf, err := manifest.ParseMarketplaceJSON(f)
	if err != nil {
		return nil, nil, err
	}

	paths := manifest.MarketplaceSkillPaths(mf)
	var skills []importedSkill
	for _, relPath := range paths {
		absPath := filepath.Join(rootDir, relPath)
		if !dirExistsAt(absPath) {
			continue // skip non-existent directories
		}
		skill, _ := parseSkillAt(rootDir, relPath, "SKILL.md")
		if skill.name == "" {
			skill.name = slugify(filepath.Base(relPath))
		}
		skill.dir = relPath
		skills = append(skills, skill)
	}

	globs := inferMemberGlobs(skills)
	return skills, globs, nil
}

func scanSkillDirs(rootDir, prefix string) []importedSkill {
	pattern := filepath.Join(rootDir, prefix, "*", "SKILL.md")
	matches, _ := filepath.Glob(pattern)

	var skills []importedSkill
	for _, match := range matches {
		skillDir := filepath.Dir(match)
		relDir, _ := filepath.Rel(rootDir, skillDir)
		skill, _ := parseSkillAt(rootDir, relDir, "SKILL.md")
		if skill.name == "" {
			skill.name = slugify(filepath.Base(skillDir))
		}
		skill.dir = relDir
		skills = append(skills, skill)
	}
	return skills
}

func scanSkillGlob(rootDir, pattern string) []importedSkill {
	matches, _ := filepath.Glob(filepath.Join(rootDir, pattern))

	var skills []importedSkill
	for _, match := range matches {
		skillDir := filepath.Dir(match)
		relDir, _ := filepath.Rel(rootDir, skillDir)

		// Skip directories that are clearly source code, not skill packages.
		// Check every component of the path, not just the leaf.
		if containsExcludedDir(relDir) {
			continue
		}

		base := filepath.Base(skillDir)
		skill, _ := parseSkillAt(rootDir, relDir, "SKILL.md")
		if skill.name == "" {
			skill.name = slugify(base)
		}
		skill.dir = relDir
		skills = append(skills, skill)
	}
	return skills
}

func scanBareMarkdown(rootDir string) []importedSkill {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	var skills []importedSkill
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		// Skip common non-skill markdown files
		lower := strings.ToLower(name)
		if lower == "readme.md" || lower == "changelog.md" || lower == "contributing.md" ||
			lower == "license.md" || lower == "claude.md" || lower == "spec.md" ||
			lower == "tasks.md" || lower == "description.md" {
			continue
		}

		// Check if it has frontmatter — if so, it would've been caught by Format 3
		f, err := os.Open(filepath.Join(rootDir, name))
		if err != nil {
			continue
		}
		fm, _, _ := manifest.ParseSkillMD(f)
		_ = f.Close()

		if fm != nil {
			continue // has frontmatter, not "bare"
		}

		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		skills = append(skills, importedSkill{
			name:  slugify(baseName),
			dir:   ".",
			entry: name,
		})
		// Only take the first bare markdown to avoid false positives
		break
	}
	return skills
}

// parseSkillAt reads a SKILL.md at rootDir/relDir/entry and extracts metadata.
func parseSkillAt(rootDir, relDir, entry string) (importedSkill, bool) {
	fullPath := filepath.Join(rootDir, relDir, entry)
	f, err := os.Open(fullPath)
	if err != nil {
		return importedSkill{dir: relDir, entry: entry}, false
	}
	defer func() { _ = f.Close() }()

	fm, _, err := manifest.ParseSkillMD(f)
	if err != nil || fm == nil {
		return importedSkill{
			name:  slugify(filepath.Base(relDir)),
			dir:   relDir,
			entry: entry,
		}, false
	}

	name := fm.Name
	if name == "" {
		name = slugify(filepath.Base(relDir))
	} else {
		name = slugify(name)
	}

	return importedSkill{
		name:           name,
		description:    fm.Description,
		dir:            relDir,
		entry:          entry,
		tags:           fm.Triggers,
		hasFrontmatter: true,
	}, true
}

// inferMemberGlobs determines the workspace member glob patterns from detected skills.
// Returns one glob per unique top-level prefix (e.g., ["engineering/*", "writing/*"]).
func inferMemberGlobs(skills []importedSkill) []string {
	if len(skills) == 0 {
		return []string{"*"}
	}

	// Group skills by their top-level prefix directory
	type prefixInfo struct {
		prefix    string
		twoLevel  int
		total     int
		order     int // insertion order for stable output
	}
	prefixes := make(map[string]*prefixInfo)
	insertOrder := 0

	for _, s := range skills {
		parts := strings.SplitN(s.dir, "/", 2)
		var key string
		if len(parts) >= 2 {
			key = parts[0]
		}
		info, ok := prefixes[key]
		if !ok {
			info = &prefixInfo{prefix: key, order: insertOrder}
			insertOrder++
			prefixes[key] = info
		}
		info.total++
		if strings.Count(s.dir, "/") >= 2 {
			info.twoLevel++
		}
	}

	// If all skills are at root level (no parent directory), return single "*"
	if len(prefixes) == 1 {
		for k := range prefixes {
			if k == "" || k == "." {
				return []string{"*"}
			}
		}
	}

	// Sort by insertion order for stable output
	sorted := make([]*prefixInfo, 0, len(prefixes))
	for _, info := range prefixes {
		sorted = append(sorted, info)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].order < sorted[j].order
	})

	var globs []string
	for _, info := range sorted {
		if info.prefix == "" || info.prefix == "." {
			globs = append(globs, "*")
		} else if info.twoLevel > info.total/2 {
			globs = append(globs, info.prefix+"/*/*")
		} else {
			globs = append(globs, info.prefix+"/*")
		}
	}
	return globs
}

// writeManifest marshals and writes a manifest to path.
func writeManifest(path string, m *manifest.Manifest, w *output.Writer) (int, error) {
	data, err := manifest.Marshal(m)
	if err != nil {
		return 0, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}

	w.PrintDim("  Wrote %s", path)
	return 1, nil
}

func fileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// excludedDirs are directories that should never be treated as skill packages.
// These are source code, build artifacts, or infrastructure directories.
var excludedDirs = map[string]bool{
	"internal":     true,
	"cmd":          true,
	"pkg":          true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true, // Rust/Java
	"__pycache__":  true,
	"docs":         true,
	"test":         true,
	"tests":        true,
	"fixtures":     true,
	"testdata":     true,
	"examples":     true,
	".github":      true,
	".claude":      true,
	".vscode":      true,
}

// containsExcludedDir checks if any path component is an excluded directory.
func containsExcludedDir(relPath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
		if excludedDirs[part] {
			return true
		}
	}
	return false
}

// deduplicateSkills removes skills with the same name, keeping the first occurrence.
func deduplicateSkills(skills []importedSkill) []importedSkill {
	seen := make(map[string]bool)
	var unique []importedSkill
	for _, s := range skills {
		if !seen[s.name] {
			seen[s.name] = true
			unique = append(unique, s)
		}
	}
	return unique
}
