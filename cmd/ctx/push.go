package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [path]",
	Short: "Push current directory as a private package",
	Long: `Push the current directory to the registry as a private, mutable package.

This is a shorthand for: ctx publish --visibility private --mutable

Examples:
  ctx push                    Push current dir as private package
  ctx push ./my-skill         Push a specific directory`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in — run 'ctx login' first")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Check for ctx.yaml or SKILL.md
		needsWrite := false
		yamlPath := filepath.Join(dir, manifest.FileName)
		var m *manifest.Manifest
		if _, statErr := os.Stat(yamlPath); statErr == nil {
			// ctx.yaml exists — must parse successfully
			var loadErr error
			m, loadErr = manifest.LoadFromDir(dir)
			if loadErr != nil {
				return fmt.Errorf("ctx.yaml exists but cannot be parsed: %w\nFix the syntax errors before pushing", loadErr)
			}
		} else {
			// No ctx.yaml — check for SKILL.md for auto-init
			skillPath := filepath.Join(dir, "SKILL.md")
			if _, skillStatErr := os.Stat(skillPath); skillStatErr != nil {
				return output.ErrUsageHint(
					"no ctx.yaml or SKILL.md found",
					"Run 'ctx init' to create a manifest, or create a SKILL.md",
				)
			}

			// Auto-init from SKILL.md
			output.Info("Found SKILL.md, auto-creating ctx.yaml...")
			scope := cfg.Username
			if scope == "" {
				scope = "your-scope"
			}
			dirName := filepath.Base(dir)
			if abs, absErr := filepath.Abs(dir); absErr == nil {
				dirName = filepath.Base(abs)
			}

			m = manifest.Scaffold(manifest.TypeSkill, scope, dirName)
			m.Visibility = "private"
			m.Mutable = true
			needsWrite = true
		}

		// Set push defaults
		if m.Visibility == "" {
			m.Visibility = "private"
			needsWrite = true
		}
		if m.Visibility == "private" && !m.Mutable {
			m.Mutable = true
			needsWrite = true
		}

		// Auto-fill scope from logged-in user
		if scope := m.Scope(); scope == "your-scope" || scope == "" {
			if cfg.Username != "" {
				_, name := manifest.ParseFullName(m.Name)
				m.Name = manifest.FormatFullName(cfg.Username, name)
				needsWrite = true
			}
		}

		// Only write ctx.yaml if fields were modified
		data, err := manifest.Marshal(m)
		if err != nil {
			return err
		}
		if needsWrite {
			if writeErr := os.WriteFile(filepath.Join(dir, manifest.FileName), data, 0o644); writeErr != nil {
				return fmt.Errorf("write %s: %w", manifest.FileName, writeErr)
			}
		}

		// Validate
		errs := manifest.Validate(m)
		if len(errs) > 0 {
			return output.ErrUsageHint("validation failed: "+errs[0], "Fix errors and try again")
		}

		reg := registry.New(cfg.RegistryURL(), token)
		output.Info("Pushing %s...", m.Name)

		// Open archive if exists
		var archive *os.File
		archivePath := filepath.Join(dir, "package.tar.gz")
		if f, openErr := os.Open(archivePath); openErr == nil {
			archive = f
			defer func() { _ = archive.Close() }()
		}

		result, err := reg.Publish(cmd.Context(), data, archive)
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Pushed %s@%s (private)", result.FullName, result.Version)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "install", Command: "ctx install " + result.FullName, Description: "Install on another device"},
				output.Breadcrumb{Action: "sync", Command: "ctx sync push", Description: "Sync all packages to profile"},
			),
		)
	},
}
