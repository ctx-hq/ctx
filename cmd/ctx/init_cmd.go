package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var initType string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ctx.yaml in the current directory",
	Long: `Scaffold a new ctx.yaml manifest for a skill, MCP server, or CLI tool.

Supports three modes:
  1. From scratch (no ctx.yaml, no SKILL.md) — scaffold with smart defaults
  2. From SKILL.md — auto-extract metadata from existing SKILL.md
  3. Update mode — supplement missing fields in existing ctx.yaml

Examples:
  ctx init                    Interactive scaffold
  ctx init --type skill       Create skill manifest
  ctx init --type mcp         Create MCP server manifest
  ctx init --type cli         Create CLI tool manifest
  ctx init -y                 Non-interactive with defaults`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		dir := "."

		// Determine scope from logged-in user
		scope := "your-scope"
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			output.Warn("Could not load config: %v", cfgErr)
		} else if cfg.Username != "" {
			scope = cfg.Username
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		dirName := filepath.Base(cwd)

		// Check if ctx.yaml exists
		yamlPath := filepath.Join(dir, manifest.FileName)
		if _, statErr := os.Stat(yamlPath); statErr == nil {
			// File exists — try to parse it
			existing, loadErr := manifest.LoadFromDir(dir)
			if loadErr != nil {
				return fmt.Errorf("ctx.yaml exists but cannot be parsed: %w\nFix the syntax errors or remove the file before running 'ctx init'", loadErr)
			}
			// Mode C: ctx.yaml already exists → supplement missing fields
			return supplementExisting(w, dir, existing, scope)
		}

		// Mode B: SKILL.md exists → extract metadata
		skillPath := filepath.Join(dir, "SKILL.md")
		if f, openErr := os.Open(skillPath); openErr == nil {
			defer func() { _ = f.Close() }()
			fm, _, parseErr := manifest.ParseSkillMD(f)
			if parseErr == nil && fm != nil {
				return initFromSkillMD(w, dir, fm, scope, dirName)
			}
			// SKILL.md exists but no parseable frontmatter — fall through to Mode A
		}

		// Mode A: From scratch
		pkgType := manifest.PackageType(initType)
		if !pkgType.Valid() {
			return output.ErrUsageHint(
				fmt.Sprintf("invalid type %q", initType),
				"Must be skill, mcp, or cli",
			)
		}

		m := manifest.Scaffold(pkgType, scope, dirName)

		// Set visibility default for --yes mode
		if flagYes {
			m.Visibility = "public"
		}

		data, err := manifest.Marshal(m)
		if err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(dir, manifest.FileName), data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", manifest.FileName, err)
		}

		return w.OK(
			map[string]string{"file": manifest.FileName, "type": initType, "mode": "scaffold"},
			output.WithSummary("Created "+manifest.FileName+" (type: "+initType+")"),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "validate", Command: "ctx validate", Description: "Validate the manifest"},
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish to registry"},
			),
		)
	},
}

// initFromSkillMD creates ctx.yaml by extracting metadata from existing SKILL.md.
func initFromSkillMD(w *output.Writer, dir string, fm *manifest.SkillFrontmatter, scope, dirName string) error {
	// Use SKILL.md name if available, fallback to directory name
	name := dirName
	if fm.Name != "" {
		name = fm.Name
	}

	m := manifest.Scaffold(manifest.TypeSkill, scope, name)
	m.Description = fm.Description

	// Map triggers to keywords
	if len(fm.Triggers) > 0 {
		m.Keywords = fm.Triggers
	}

	if m.Skill == nil {
		m.Skill = &manifest.SkillSpec{}
	}
	m.Skill.Entry = "SKILL.md"
	if fm.Invocable {
		invocable := true
		m.Skill.UserInvocable = &invocable
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", manifest.FileName, err)
	}

	output.Info("Extracted from SKILL.md: name=%s, %d triggers", fm.Name, len(fm.Triggers))

	return w.OK(
		map[string]string{"file": manifest.FileName, "type": "skill", "mode": "from-skillmd"},
		output.WithSummary("Created "+manifest.FileName+" from SKILL.md"),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "validate", Command: "ctx validate", Description: "Validate"},
			output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish"},
		),
	)
}

// supplementExisting fills missing fields in an existing ctx.yaml.
func supplementExisting(w *output.Writer, dir string, m *manifest.Manifest, scope string) error {
	changed := 0

	// Auto-fill scope if placeholder
	if s := m.Scope(); s == "your-scope" || s == "" {
		_, name := manifest.ParseFullName(m.Name)
		m.Name = manifest.FormatFullName(scope, name)
		changed++
	}

	// Fill missing description
	if m.Description == "" {
		m.Description = fmt.Sprintf("A %s package", m.Type)
		changed++
	}

	// Fill visibility if missing
	if m.Visibility == "" {
		m.Visibility = "public"
		changed++
	}

	// Supplement skill entry if missing
	if m.Type == manifest.TypeSkill && m.Skill != nil && m.Skill.Entry == "" {
		skillPath := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			m.Skill.Entry = "SKILL.md"
			changed++
		}
	}

	if changed == 0 {
		return w.OK(
			map[string]string{"file": manifest.FileName, "mode": "update", "changes": "0"},
			output.WithSummary(manifest.FileName+" is already complete, no changes needed"),
		)
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", manifest.FileName, err)
	}

	return w.OK(
		map[string]string{"file": manifest.FileName, "mode": "update", "changes": fmt.Sprintf("%d", changed)},
		output.WithSummary(fmt.Sprintf("Updated %s (%d fields added)", manifest.FileName, changed)),
	)
}

func init() {
	initCmd.Flags().StringVarP(&initType, "type", "t", "skill", "Package type (skill, mcp, cli)")
}
