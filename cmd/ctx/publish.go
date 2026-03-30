package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/publishcheck"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Publish a package to the registry",
	Long: `Publish a package defined by ctx.yaml to getctx.org.

Reads ctx.yaml from the current directory (or specified path),
validates it, and uploads to the registry.

Accepts a directory with ctx.yaml, or a single .md file (auto-scaffolds into
a standard skill package with interactive prompts).

Examples:
  ctx publish                    Publish current dir
  ctx publish ./my-skill         Publish a specific directory
  ctx publish gc.md              Publish a single .md file as a skill`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)

		// Detect single-file input
		if len(args) > 0 && isSingleFile(args[0]) {
			return pushSingleFile(cmd, args[0], w, singleFileOpts{
				defaultVisibility: "public",
				mutable:           false,
				versionBump:       flagBump,
				skipConfirm:       flagYes,
			})
		}

		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		// Load and validate manifest
		m, err := manifest.LoadFromDir(dir)
		if err != nil {
			return err
		}

		// Apply version bump if --bump is set
		if flagBump != "" {
			bumped, bumpErr := manifest.BumpVersion(m.Version, flagBump)
			if bumpErr != nil {
				return bumpErr
			}
			m.Version = bumped
			// Write back updated version
			bumpData, marshalErr := manifest.Marshal(m)
			if marshalErr != nil {
				return marshalErr
			}
			if writeErr := os.WriteFile(filepath.Join(dir, manifest.FileName), bumpData, 0o644); writeErr != nil {
				return fmt.Errorf("write %s: %w", manifest.FileName, writeErr)
			}
		}

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			return output.ErrUsageHint(
				"validation failed: "+errs[0],
				"Fix errors and try again",
			)
		}

		// Validate install methods for CLI packages
		var checkResults []publishcheck.CheckResult
		if m.Type == manifest.TypeCLI && m.Install != nil && !flagForce {
			checkResults = publishcheck.Check(cmd.Context(), m)
			issues := 0
			for _, r := range checkResults {
				if !r.OK {
					output.Warn("install.%s: %s — %s", r.Method, r.Pkg, r.Error)
					issues++
				}
			}
			if issues > 0 {
				return output.ErrUsageHint(
					fmt.Sprintf("%d install method(s) failed validation", issues),
					"Fix your ctx.yaml or use --force to publish anyway",
				)
			}
		}

		// Pre-publish checklist (TTY only)
		if !flagYes {
			checklist := publishcheck.FormatChecklist(m, checkResults)
			fmt.Fprintln(os.Stderr, checklist)
			p := prompt.DefaultPrompter()
			confirmed, pErr := p.Confirm("Publish?", true)
			if pErr != nil || !confirmed {
				output.Info("Cancelled.")
				return nil
			}
		}

		// Check auth
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if getToken() == "" {
			return output.ErrAuth("not logged in")
		}

		// Marshal manifest
		data, err := manifest.Marshal(m)
		if err != nil {
			return err
		}

		// Publish
		reg := registry.New(cfg.RegistryURL(), getToken())

		output.Info("Publishing %s@%s...", m.Name, m.Version)

		// Stage only package files (whitelist) and create archive.
		stg, stgErr := staging.New("ctx-publish-")
		if stgErr != nil {
			return stgErr
		}
		defer stg.Rollback()

		if cpErr := stg.CopyFiles(dir, m.PackageFiles()); cpErr != nil {
			return fmt.Errorf("stage package files: %w", cpErr)
		}

		// Normalize: copy non-root skill entry to root SKILL.md for install-side compat.
		if m.SkillEntryNeedsNormalize() {
			if err := stg.NormalizeSkillEntry(m.Skill.Entry); err != nil {
				return fmt.Errorf("normalize skill entry: %w", err)
			}
		}

		if wErr := stg.WriteFile(manifest.FileName, data, 0o644); wErr != nil {
			return fmt.Errorf("stage manifest: %w", wErr)
		}

		archive, archErr := stg.TarGz()
		if archErr != nil {
			return fmt.Errorf("create archive: %w", archErr)
		}
		defer func() { _ = archive.Close() }()

		result, err := reg.Publish(cmd.Context(), data, archive)
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary("Published "+result.FullName+"@"+result.Version),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info " + result.FullName, Description: "View package"},
			),
		)
	},
}
