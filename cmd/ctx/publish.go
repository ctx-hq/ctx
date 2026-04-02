package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/license"
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

		// Workspace mode: publish all members.
		if m.Type == manifest.TypeWorkspace {
			if !flagPublishAll && flagPublishFilter == "" {
				return output.ErrUsageHint(
					"cannot publish a workspace directly",
					"Use --all to publish all members, or --filter to publish matching members",
				)
			}
			return publishWorkspace(cmd, dir, w)
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

		// Apply --private flag
		if flagPrivate {
			m.Visibility = "private"
		}

		// Auto-enrich missing metadata from git/filesystem
		autoEnrichManifest(m, dir)

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

		// Read README.md for inclusion in publish
		readmeData, _ := os.ReadFile(filepath.Join(dir, "README.md"))

		result, err := reg.Publish(cmd.Context(), data, archive, readmeData)
		if err != nil {
			return err
		}

		pubBreadcrumbs := []output.Breadcrumb{
			{Action: "info", Command: "ctx info " + result.FullName, Description: "View package"},
		}
		if m.Type == manifest.TypeCLI {
			pubBreadcrumbs = append(pubBreadcrumbs,
				output.Breadcrumb{Action: "upload", Command: "ctx artifact upload " + result.FullName + "@" + result.Version + " --dir dist/", Description: "Upload platform binaries"},
			)
		}
		return w.OK(result,
			output.WithSummary("Published "+result.FullName+"@"+result.Version),
			output.WithBreadcrumbs(pubBreadcrumbs...),
		)
	},
}

// publishWorkspace publishes all (or filtered) workspace members sequentially.
func publishWorkspace(cmd *cobra.Command, dir string, w *output.Writer) error {
	ws, err := manifest.LoadWorkspace(dir)
	if err != nil {
		return err
	}

	// Check auth once before the loop.
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := getToken()
	if token == "" {
		return output.ErrAuth("not logged in")
	}
	reg := registry.New(cfg.RegistryURL(), token)

	// Apply filter if specified.
	members := ws.Members
	if flagPublishFilter != "" {
		var filtered []*manifest.WorkspaceMember
		for _, m := range members {
			matched, _ := filepath.Match(flagPublishFilter, filepath.Base(m.Dir))
			matchedRel, _ := filepath.Match(flagPublishFilter, m.RelDir)
			_, shortName := manifest.ParseFullName(m.Manifest.Name)
			matchedName, _ := filepath.Match(flagPublishFilter, shortName)
			if matched || matchedRel || matchedName {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) == 0 {
			return output.ErrUsageHint(
				fmt.Sprintf("no members matched filter %q", flagPublishFilter),
				"Use 'ctx workspace list' to see available members",
			)
		}
		members = filtered
	}

	output.Info("Publishing %d member(s) from workspace %s...", len(members), ws.Root.Name)

	var published int
	var failed int
	for i, member := range members {
		output.Info("")
		output.Info("[%d/%d] Publishing %s@%s...", i+1, len(members), member.Manifest.Name, member.Manifest.Version)

		// Publish each member by invoking the existing publish flow.
		pubErr := publishSingleMember(cmd, member, reg)
		if pubErr != nil {
			failed++
			output.Warn("Failed: %v", pubErr)
			if !flagPublishContinueOnErr {
				return fmt.Errorf("publish %s failed: %w (use --continue-on-error to skip failures)", member.Manifest.Name, pubErr)
			}
			continue
		}
		published++
	}

	output.Info("")
	if failed > 0 {
		output.Warn("Published %d/%d members (%d failed)", published, len(members), failed)
	} else {
		output.Info("Published %d member(s) from workspace %s", published, ws.Root.Name)
	}

	return nil
}

// publishSingleMember publishes a single workspace member.
func publishSingleMember(cmd *cobra.Command, member *manifest.WorkspaceMember, reg *registry.Client) error {
	m := member.Manifest
	dir := member.Dir

	// Auto-enrich (quiet mode: skip warnings already shown or redundant in batch)
	autoEnrichManifestQuiet(m, dir)

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		return output.ErrUsageHint(
			"validation failed: "+errs[0],
			"Fix errors and try again",
		)
	}

	// Marshal manifest
	data, err := manifest.Marshal(m)
	if err != nil {
		return err
	}

	// Stage package files
	stg, stgErr := staging.New("ctx-publish-")
	if stgErr != nil {
		return stgErr
	}
	defer stg.Rollback()

	if cpErr := stg.CopyFiles(dir, m.PackageFiles()); cpErr != nil {
		return fmt.Errorf("stage package files: %w", cpErr)
	}

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

	result, err := reg.Publish(cmd.Context(), data, archive, nil)
	if err != nil {
		return err
	}

	output.Success("Published %s@%s", result.FullName, result.Version)
	return nil
}

// autoEnrichManifest fills in missing optional metadata fields from
// git configuration and filesystem detection. It modifies m in place.
// Fields that already have values are never overwritten.
func autoEnrichManifest(m *manifest.Manifest, dir string) {
	enrichManifest(m, dir, false)
}

// autoEnrichManifestQuiet is like autoEnrichManifest but suppresses
// per-field warnings. Used in workspace batch publish to avoid noisy
// repeated warnings for every member.
func autoEnrichManifestQuiet(m *manifest.Manifest, dir string) {
	enrichManifest(m, dir, true)
}

func enrichManifest(m *manifest.Manifest, dir string, quiet bool) {
	if m.Author == "" {
		if author := gitutil.Author(dir); author != "" {
			m.Author = author
			if !quiet {
				output.Info("Auto-detected author: %s", author)
			}
		}
	}
	if m.Repository == "" {
		if repo := gitutil.RemoteURL(dir); repo != "" {
			m.Repository = repo
			if !quiet {
				output.Info("Auto-detected repository: %s", repo)
			}
		}
	}
	if m.License == "" {
		if lr := license.Detect(dir); lr.SPDX != "" {
			m.License = lr.SPDX
			if !quiet {
				output.Info("Auto-detected license: %s", lr.SPDX)
			}
		}
	}

	if quiet {
		return
	}

	// Soft warnings for missing recommended fields
	if m.Repository == "" {
		output.Warn("No repository URL detected (add to ctx.yaml or configure git remote)")
	}
	if m.License == "" {
		output.Warn("No license detected (add LICENSE file or set license in ctx.yaml)")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); os.IsNotExist(err) {
		output.Warn("No README.md found (recommended for package documentation)")
	}
}
