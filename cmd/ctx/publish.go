package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/license"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/publishcheck"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/securityscan"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

var errUpgradeCancelled = errors.New("upgrade cancelled")

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
			vis := "public"
			if flagPrivate {
				vis = "private"
			}
			return publishSingleFile(cmd, args[0], w, singleFileOpts{
				defaultVisibility: vis,
				versionBump:       flagBump,
				publishTag:        flagPublishTag,
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

		// Apply --version: override version from ctx.yaml (for CI: git tag version)
		if flagVersion != "" {
			m.Version = flagVersion
		}

		// Apply --tag: append prerelease suffix (temporary, does not modify ctx.yaml)
		if flagPublishTag != "" {
			m.Version = appendPrerelease(m.Version, flagPublishTag)
			w.Info("Publishing as prerelease: %s", m.Version)
		}

		// Apply --private flag
		if flagPrivate {
			m.Visibility = "private"
		}

		// Auto-enrich missing metadata from git/filesystem
		autoEnrichManifest(m, dir, w)

		// Auto-fill description/keywords from SKILL.md frontmatter
		enrichFromSkillMD(m, dir, w)

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
					w.Warn("install.%s: %s — %s", r.Method, r.Pkg, r.Error)
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

		// Security scan
		scanResult, scanErr := securityscan.Scan(dir)
		if scanErr != nil {
			w.Warn("Security scan error: %v", scanErr)
		} else if len(scanResult.Findings) > 0 {
			for _, f := range scanResult.Findings {
				switch f.Severity {
				case securityscan.Critical:
					w.Warn("[CRITICAL] %s:%d — %s", f.File, f.Line, f.Message)
				case securityscan.High:
					w.Warn("[HIGH] %s:%d — %s", f.File, f.Line, f.Message)
				case securityscan.Medium:
					w.Info("[MEDIUM] %s:%d — %s", f.File, f.Line, f.Message)
				}
			}
			if !scanResult.Passed() && !flagForce {
				severity := "high-severity"
				if scanResult.HasCritical() {
					severity = "critical"
				}
				return output.ErrUsageHint(
					fmt.Sprintf("security scan found %s issues", severity),
					"Review findings above. Use --force to publish anyway",
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
				w.Info("Cancelled.")
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

		// Auto-upgrade private→target visibility if needed.
		if err := maybeUpgradeVisibility(cmd.Context(), reg, w, m.Name, m.Visibility, flagYes); err != nil {
			if errors.Is(err, errUpgradeCancelled) {
				w.Info("Cancelled.")
				return nil
			}
			return err
		}

		w.Info("Publishing %s@%s...", m.Name, m.Version)

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

		packageURL := cfg.PackageWebURL(result.FullName)
		pubBreadcrumbs := []output.Breadcrumb{
			{Action: "view", Command: packageURL, Description: "View on getctx.org"},
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

	// Apply --changed filter: only publish members with git changes.
	members := ws.Members
	if cmd.Flags().Changed("changed") {
		ref := flagPublishChanged
		if ref == "" {
			ref = "HEAD~1"
		}
		// Collect member relative dirs for git diff
		var memberDirs []string
		for _, m := range members {
			memberDirs = append(memberDirs, m.RelDir)
		}
		changedDirs := gitutil.ChangedDirs(dir, ref, memberDirs)
		changedSet := make(map[string]bool, len(changedDirs))
		for _, d := range changedDirs {
			changedSet[d] = true
		}
		var changed []*manifest.WorkspaceMember
		for _, m := range members {
			if changedSet[m.RelDir] {
				changed = append(changed, m)
			}
		}
		if len(changed) == 0 {
			w.Info("No changed skills to publish (compared to %s)", ref)
			return nil
		}
		w.Info("Detected %d changed member(s) (compared to %s)", len(changed), ref)
		members = changed
	}

	// Apply filter if specified.
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

	w.Info("Publishing %d member(s) from workspace %s...", len(members), ws.Root.Name)

	var published int
	var failed int
	for i, member := range members {
		w.Info("")
		w.Info("[%d/%d] Publishing %s@%s...", i+1, len(members), member.Manifest.Name, member.Manifest.Version)

		// Publish each member by invoking the existing publish flow.
		pubErr := publishSingleMember(cmd, member, reg, w)
		if pubErr != nil {
			failed++
			w.Warn("Failed: %v", pubErr)
			if !flagPublishContinueOnErr {
				return fmt.Errorf("publish %s failed: %w (use --continue-on-error to skip failures)", member.Manifest.Name, pubErr)
			}
			continue
		}
		published++
	}

	w.Info("")
	if failed > 0 {
		w.Warn("Published %d/%d members (%d failed)", published, len(members), failed)
	} else {
		w.Info("Published %d member(s) from workspace %s", published, ws.Root.Name)
	}

	return nil
}

// publishSingleMember publishes a single workspace member.
func publishSingleMember(cmd *cobra.Command, member *manifest.WorkspaceMember, reg *registry.Client, w *output.Writer) error {
	m := member.Manifest
	dir := member.Dir

	// Auto-enrich (quiet mode: skip warnings already shown or redundant in batch)
	autoEnrichManifestQuiet(m, dir)

	// Auto-fill description/keywords from SKILL.md frontmatter (quiet)
	enrichFromSkillMD(m, dir, nil)

	// Apply --tag prerelease suffix
	if flagPublishTag != "" {
		m.Version = appendPrerelease(m.Version, flagPublishTag)
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		return output.ErrUsageHint(
			"validation failed: "+errs[0],
			"Fix errors and try again",
		)
	}

	// Security scan (same gate as single-package publish)
	scanResult, scanErr := securityscan.Scan(dir)
	if scanErr != nil {
		w.Warn("Security scan error for %s: %v", m.Name, scanErr)
	} else if len(scanResult.Findings) > 0 {
		for _, f := range scanResult.Findings {
			switch f.Severity {
			case securityscan.Critical:
				w.Warn("[CRITICAL] %s/%s:%d — %s", m.Name, f.File, f.Line, f.Message)
			case securityscan.High:
				w.Warn("[HIGH] %s/%s:%d — %s", m.Name, f.File, f.Line, f.Message)
			case securityscan.Medium:
				w.Info("[MEDIUM] %s/%s:%d — %s", m.Name, f.File, f.Line, f.Message)
			}
		}
		if !scanResult.Passed() && !flagForce {
			severity := "high-severity"
			if scanResult.HasCritical() {
				severity = "critical"
			}
			return output.ErrUsageHint(
				fmt.Sprintf("security scan found %s issues in %s", severity, m.Name),
				"Review findings above. Use --force to publish anyway",
			)
		}
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

	// Auto-upgrade private→target visibility if needed.
	if err := maybeUpgradeVisibility(cmd.Context(), reg, w, m.Name, m.Visibility, true); err != nil {
		return err
	}

	result, err := reg.Publish(cmd.Context(), data, archive, nil)
	if err != nil {
		return err
	}

	w.Success("Published %s@%s", result.FullName, result.Version)
	return nil
}

// maybeUpgradeVisibility checks whether an existing private package needs its
// visibility upgraded before publish. It respects targetVis from the manifest:
//   - "" (unspecified) defaults to "public"
//   - "private" → no upgrade, the user wants to stay private
//   - "public" / "unlisted" → upgrade to that value
//
// NOTE: upgrade (freeze + visibility change) and publish are not atomic.
// If publish fails after upgrade, the package visibility will already be changed.
func maybeUpgradeVisibility(ctx context.Context, reg *registry.Client, w *output.Writer, fullName string, targetVis string, skipConfirm bool) error {
	if targetVis == "" {
		targetVis = "public"
	}
	if targetVis == "private" {
		return nil
	}

	existing, err := reg.GetPackage(ctx, fullName)
	if err != nil {
		if registry.IsNotFound(err) {
			return nil
		}
		return err
	}

	if existing.Visibility != "private" {
		return nil
	}

	if !skipConfirm {
		p := prompt.DefaultPrompter()
		confirmed, pErr := p.Confirm(
			fmt.Sprintf("Package %s is currently private. Make it %s?", fullName, targetVis),
			true,
		)
		if pErr != nil {
			return pErr
		}
		if !confirmed {
			return errUpgradeCancelled
		}
	}

	w.Info("Making %s %s...", fullName, targetVis)
	if err := reg.SetVisibility(ctx, fullName, targetVis); err != nil {
		return fmt.Errorf("change visibility: %w", err)
	}

	return nil
}

// enrichFromSkillMD reads the SKILL.md frontmatter and fills in missing
// manifest fields (description, keywords). This allows ctx.yaml to be minimal
// — authors don't need to duplicate what SKILL.md already declares.
// Fields that already have values are never overwritten.
func enrichFromSkillMD(m *manifest.Manifest, dir string, w *output.Writer) {
	if m.Type != manifest.TypeSkill {
		return
	}
	if m.Skill == nil || m.Skill.Entry == "" {
		return
	}

	skillPath := filepath.Join(dir, m.Skill.Entry)
	f, err := os.Open(skillPath)
	if err != nil {
		return // SKILL.md not found is not an error here
	}
	defer func() { _ = f.Close() }()

	fm, _, parseErr := manifest.ParseSkillMD(f)
	if parseErr != nil || fm == nil {
		return
	}

	if m.Description == "" && fm.Description != "" {
		desc := fm.Description
		if runes := []rune(desc); len(runes) > 1024 {
			desc = string(runes[:1021]) + "..."
		}
		m.Description = desc
		if w != nil {
			w.Info("Auto-filled description from SKILL.md frontmatter")
		}
	}

	// Don't auto-fill keywords from triggers — they serve different purposes.
}

// autoEnrichManifest fills in missing optional metadata fields from
// git configuration and filesystem detection. It modifies m in place.
// Fields that already have values are never overwritten.
func autoEnrichManifest(m *manifest.Manifest, dir string, w *output.Writer) {
	enrichManifest(m, dir, false, w)
}

// autoEnrichManifestQuiet is like autoEnrichManifest but suppresses
// per-field warnings. Used in workspace batch publish to avoid noisy
// repeated warnings for every member.
func autoEnrichManifestQuiet(m *manifest.Manifest, dir string) {
	enrichManifest(m, dir, true, nil)
}

func enrichManifest(m *manifest.Manifest, dir string, quiet bool, w *output.Writer) {
	if m.Author == "" {
		if author := gitutil.Author(dir); author != "" {
			m.Author = author
			if !quiet && w != nil {
				w.Info("Auto-detected author: %s", author)
			}
		}
	}
	if m.Repository == "" {
		if repo := gitutil.RemoteURL(dir); repo != "" {
			m.Repository = repo
			if !quiet && w != nil {
				w.Info("Auto-detected repository: %s", repo)
			}
		}
	}
	if m.License == "" {
		if lr := license.Detect(dir); lr.SPDX != "" {
			m.License = lr.SPDX
			if !quiet && w != nil {
				w.Info("Auto-detected license: %s", lr.SPDX)
			}
		}
	}

	if quiet || w == nil {
		return
	}

	// Soft warnings for missing recommended fields
	if m.Repository == "" {
		w.Warn("No repository URL detected (add to ctx.yaml or configure git remote)")
	}
	if m.License == "" {
		w.Warn("No license detected (add LICENSE file or set license in ctx.yaml)")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); os.IsNotExist(err) {
		w.Warn("No README.md found (recommended for package documentation)")
	}
}

// appendPrerelease appends a prerelease tag with a timestamp to a semver version.
// e.g., appendPrerelease("1.2.0", "canary") → "1.2.0-canary.20260404120000"
// If the version already has a prerelease suffix, it is replaced.
// The ctx.yaml file is NOT modified — this is a temporary in-memory override.
func appendPrerelease(version, tag string) string {
	ts := time.Now().UTC().Format("20060102150405")
	// Strip existing prerelease/metadata: take only "major.minor.patch"
	if idx := strings.IndexAny(version, "-+"); idx != -1 {
		version = version[:idx]
	}
	return fmt.Sprintf("%s-%s.%s", version, tag, ts)
}
