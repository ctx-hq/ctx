package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/importer"
	"github.com/ctx-hq/ctx/internal/license"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// runInitImport is the entry point for `ctx init --import`.
func runInitImport(cmd *cobra.Command, w *output.Writer) error {
	dir := "."
	args := cmd.Flags().Args()
	if len(args) > 0 {
		dir = args[0]
	}

	det, err := importer.DetectLayout(dir)
	if err != nil {
		return err
	}

	if det.Format == importer.FormatUnknown {
		return output.ErrUsageHint(
			"could not detect a skill repo format in this directory",
			"Ensure the directory contains SKILL.md files or .claude-plugin/marketplace.json",
		)
	}

	w.Info("Detected format: %s (%d skill(s))", det.Format, len(det.Skills))

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

	switch det.Format {
	case importer.FormatSingleSkill:
		skill := det.Skills[0]
		n, writeErr := writeSingleSkillCtxYaml(absDir, skill, scope, author, lic, repo, w)
		if writeErr != nil {
			return writeErr
		}
		filesWritten = n

	case importer.FormatBareMarkdown:
		skill := det.Skills[0]
		if skill.Name == "" {
			if term.IsTerminal(int(os.Stdin.Fd())) && !flagYes {
				p := prompt.DefaultPrompter()
				skill.Name, err = p.Text("Skill name", importer.Slugify(filepath.Base(absDir)))
				if err != nil {
					return err
				}
				skill.Description, err = p.Text("Description", "")
				if err != nil {
					return err
				}
			} else {
				skill.Name = importer.Slugify(filepath.Base(absDir))
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
	w.Success("Imported %d skill(s) as @%s — wrote %d file(s)", len(det.Skills), scope, filesWritten)
	w.PrintDim("")
	w.PrintDim("  Next steps:")
	w.PrintDim("    1. Review the generated ctx.yaml files")
	if len(det.Skills) > 1 {
		w.PrintDim("    2. ctx publish --all --yes     Publish all skills")
	} else {
		w.PrintDim("    2. ctx publish --yes           Publish the skill")
	}
	w.PrintDim("    3. Add .github/workflows/ctx-publish.yml for CI/CD")

	return nil
}

// writeSingleSkillCtxYaml writes a single ctx.yaml for a standalone skill repo.
func writeSingleSkillCtxYaml(rootDir string, skill importer.Skill, scope, author, lic, repo string, w *output.Writer) (int, error) {
	ctxPath := filepath.Join(rootDir, manifest.FileName)
	if importer.FileExistsAt(ctxPath) {
		w.Warn("Skipping %s (ctx.yaml already exists)", manifest.FileName)
		return 0, nil
	}

	m := buildManifest(skill, scope, rootDir, repo, "")
	applyTypeOverride(m)
	m.Author = author
	m.License = lic
	m.Repository = repo

	return writeManifest(ctxPath, m, w)
}

// writeWorkspaceImport writes workspace root ctx.yaml + per-skill ctx.yaml files,
// plus release-please config files.
func writeWorkspaceImport(rootDir string, det *importer.Detection, scope, author, lic, repo string, w *output.Writer) (int, error) {
	var total int

	// 1. Write workspace root ctx.yaml
	rootCtxPath := filepath.Join(rootDir, manifest.FileName)
	if importer.FileExistsAt(rootCtxPath) {
		w.Warn("Skipping root ctx.yaml (already exists)")
	} else {
		wsName := importer.Slugify(filepath.Base(rootDir))
		desc := fmt.Sprintf("Workspace for @%s skills", scope)
		m := &manifest.Manifest{
			Name:        manifest.FormatFullName(scope, wsName),
			Type:        manifest.TypeWorkspace,
			Description: desc,
			Workspace: &manifest.WorkspaceSpec{
				Members: det.MemberGlobs,
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
	for _, skill := range det.Skills {
		skillCtxPath := filepath.Join(rootDir, skill.Dir, manifest.FileName)
		if importer.FileExistsAt(skillCtxPath) {
			w.PrintDim("  Skipping %s (ctx.yaml already exists)", skill.Dir)
			continue
		}

		memberSkill := skill
		memberSkill.Dir = "."
		m := buildManifest(memberSkill, "", filepath.Join(rootDir, skill.Dir), repo, skill.Dir)
		m.Name = skill.Name

		n, err := writeManifest(skillCtxPath, m, w)
		if err != nil {
			return total, fmt.Errorf("write %s/ctx.yaml: %w", skill.Dir, err)
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

// buildManifest creates a Manifest for a detected skill, auto-detecting CLI projects.
func buildManifest(skill importer.Skill, scope string, rootDir string, repo string, repoRelDir string) *manifest.Manifest {
	name := skill.Name
	if scope != "" {
		name = manifest.FormatFullName(scope, skill.Name)
	}
	version := skill.Version
	if version == "" {
		version = "0.1.0"
	}

	entry := skill.Entry
	if entry == "" {
		entry = "SKILL.md"
	}
	if skill.Dir != "" && skill.Dir != "." {
		entry = filepath.Join(skill.Dir, entry)
	}

	pkgType := detectProjectType(rootDir)

	m := &manifest.Manifest{
		Name:        name,
		Version:     version,
		Type:        pkgType,
		Description: skill.Description,
		Skill:       &manifest.SkillSpec{Entry: entry},
	}

	if pkgType == manifest.TypeCLI {
		binary := skill.Name
		if grBinary, ok := detectCLIFromGoreleaser(rootDir); ok {
			binary = grBinary
		}
		m.CLI = &manifest.CLISpec{
			Binary: binary,
			Verify: binary + " --version",
		}

		if spec := detectInstallSpec(rootDir, repo, repoRelDir); spec != nil {
			m.Install = spec
		}
	}

	if len(skill.Tags) > 0 {
		if m.Skill == nil {
			m.Skill = &manifest.SkillSpec{}
		}
		tags := skill.Tags
		if len(tags) > 10 {
			tags = tags[:10]
		}
		m.Skill.Tags = tags
		m.Keywords = tags
	}
	return m
}

// applyTypeOverride applies --type flag if the user explicitly set it.
func applyTypeOverride(m *manifest.Manifest) {
	if flagInitType == "" {
		return
	}
	m.Type = manifest.PackageType(flagInitType)
	if m.Type == manifest.TypeCLI && m.CLI == nil {
		binary := m.ShortName()
		m.CLI = &manifest.CLISpec{
			Binary: binary,
			Verify: binary + " --version",
		}
	}
}

// detectProjectType checks if the directory is a CLI project or a plain skill.
func detectProjectType(dir string) manifest.PackageType {
	if importer.FileExistsAt(filepath.Join(dir, "go.mod")) && importer.DirExistsAt(filepath.Join(dir, "cmd")) {
		return manifest.TypeCLI
	}
	if importer.FileExistsAt(filepath.Join(dir, "Cargo.toml")) && importer.FileExistsAt(filepath.Join(dir, "src", "main.rs")) {
		return manifest.TypeCLI
	}
	if importer.FileExistsAt(filepath.Join(dir, "setup.py")) {
		return manifest.TypeCLI
	}
	if importer.FileExistsAt(filepath.Join(dir, ".goreleaser.yaml")) || importer.FileExistsAt(filepath.Join(dir, ".goreleaser.yml")) {
		return manifest.TypeCLI
	}
	if importer.FileExistsAt(filepath.Join(dir, "Makefile")) && importer.FileExistsAt(filepath.Join(dir, "go.mod")) {
		return manifest.TypeCLI
	}
	return manifest.TypeSkill
}

// writeReleasePleaseConfig generates release-please-config.json and
// .release-please-manifest.json for monorepo versioning.
func writeReleasePleaseConfig(rootDir string, det *importer.Detection, w *output.Writer) (int, error) {
	var written int

	configPath := filepath.Join(rootDir, "release-please-config.json")
	if !importer.FileExistsAt(configPath) {
		packages := make(map[string]interface{})
		for _, skill := range det.Skills {
			packages[skill.Dir] = map[string]interface{}{
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

	manifestPath := filepath.Join(rootDir, ".release-please-manifest.json")
	if !importer.FileExistsAt(manifestPath) {
		versions := make(map[string]string)
		for _, skill := range det.Skills {
			v := skill.Version
			if v == "" {
				v = "0.1.0"
			}
			versions[skill.Dir] = v
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

// --- CLI install detection ---

type goreleaserConfig struct {
	Builds []struct {
		Binary string `yaml:"binary"`
	} `yaml:"builds"`
}

func detectCLIFromGoreleaser(dir string) (string, bool) {
	for _, name := range []string{".goreleaser.yaml", ".goreleaser.yml"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg goreleaserConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if len(cfg.Builds) > 0 && cfg.Builds[0].Binary != "" {
			return cfg.Builds[0].Binary, true
		}
	}
	return "", false
}

func detectInstallSpec(dir, repo, repoRelDir string) *manifest.InstallSpec {
	spec := &manifest.InstallSpec{}
	hasMethod := false

	ownerRepo := githubOwnerRepo(repo)
	if ownerRepo != "" {
		spec.Source = "github:" + ownerRepo

		if importer.FileExistsAt(filepath.Join(dir, "scripts", "install.sh")) {
			branch := gitutil.DefaultBranch(dir)
			if branch == "" {
				branch = "main"
			}
			scriptPath := "scripts/install.sh"
			if repoRelDir != "" {
				scriptPath = filepath.ToSlash(filepath.Join(repoRelDir, scriptPath))
			}
			spec.Script = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", ownerRepo, branch, scriptPath)
			hasMethod = true
		}
	}

	if !hasMethod {
		return nil
	}
	return spec
}

func githubOwnerRepo(repoURL string) string {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(repoURL, prefix) {
		return ""
	}
	path := strings.TrimPrefix(repoURL, prefix)
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return path
}
