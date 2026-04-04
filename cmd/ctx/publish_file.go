package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

var flagBump string
var flagForce bool
var flagPrivate bool

var (
	flagPublishAll           bool
	flagPublishFilter        string
	flagPublishContinueOnErr bool
	flagPublishChanged       string // --changed [ref]: only publish members with changes since ref
	flagPublishTag           string // --tag canary|beta|rc: publish with prerelease suffix
)

func init() {
	publishCmd.Flags().StringVar(&flagBump, "bump", "", "Version bump strategy (patch, minor, major)")
	publishCmd.Flags().BoolVar(&flagForce, "force", false, "Skip install method validation")
	publishCmd.Flags().BoolVar(&flagPrivate, "private", false, "Publish as a private package")
	publishCmd.Flags().BoolVar(&flagPublishAll, "all", false, "Publish all workspace members")
	publishCmd.Flags().StringVar(&flagPublishFilter, "filter", "", "Glob filter for workspace members to publish")
	publishCmd.Flags().BoolVar(&flagPublishContinueOnErr, "continue-on-error", false, "Continue publishing on member failure")
	publishCmd.Flags().StringVar(&flagPublishChanged, "changed", "", "Only publish members changed since ref (default: HEAD~1 if flag set without value)")
	publishCmd.Flags().Lookup("changed").NoOptDefVal = "HEAD~1"
	publishCmd.Flags().StringVar(&flagPublishTag, "tag", "", "Publish with prerelease tag (e.g., canary, beta, rc)")
}

// singleFileOpts configures behavior for single-file skill publishing.
type singleFileOpts struct {
	defaultVisibility string // "public" (default) or "private" (--private flag)
	versionBump       string // "patch"/"minor"/"major"/""
	publishTag        string // prerelease tag (canary, beta, rc)
	skipConfirm       bool   // --yes flag
	dryRun            bool   // --dry-run flag
}

// publishSingleFile handles `ctx publish <file.md>`.
func publishSingleFile(cmd *cobra.Command, filePath string, w *output.Writer, opts singleFileOpts) error {
	// 1. Auth check
	token := getToken()
	if token == "" {
		return output.ErrAuth("not logged in — run 'ctx login' first")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	username := resolvedUsername()
	if username == "" {
		return output.ErrAuth("username not set — run 'ctx login' first")
	}

	// 2. Validate input file
	if !strings.HasSuffix(strings.ToLower(filePath), ".md") {
		return output.ErrUsageHint(
			"single-file publish supports .md files only",
			"Provide a .md file or a directory with ctx.yaml",
		)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if len(data) == 0 {
		return output.ErrUsageHint("file is empty", "Add skill content to "+filePath)
	}

	// 3. Parse frontmatter
	fm, body, parseErr := manifest.ParseSkillMD(strings.NewReader(string(data)))
	if parseErr != nil {
		w.Warn("Could not parse frontmatter: %v (treating as plain content)", parseErr)
		fm = nil
		body = string(data)
	}

	// 4. Create prompter
	var p prompt.Prompter
	if opts.skipConfirm {
		p = prompt.NoopPrompter{}
	} else {
		p = prompt.DefaultPrompter()
	}

	// 5. Derive and fill metadata
	defaultName := slugify(strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)))

	skillName := defaultName
	if fm != nil && fm.Name != "" {
		skillName = fm.Name
	}
	skillName, err = p.Text("Package name", skillName)
	if err != nil {
		return err
	}
	skillName = slugify(skillName)

	// Resolve base version from existing local package (if any)
	scope := username
	version := resolveBaseVersion(scope, skillName)
	if opts.versionBump != "" {
		bumped, bumpErr := manifest.BumpVersion(version, opts.versionBump)
		if bumpErr != nil {
			return bumpErr
		}
		version = bumped
	}
	if opts.publishTag != "" {
		version = appendPrerelease(version, opts.publishTag)
		w.Info("Publishing as prerelease: %s", version)
	}
	version, err = p.Text("Version", version)
	if err != nil {
		return err
	}

	description := ""
	if fm != nil && fm.Description != "" {
		description = fm.Description
	} else {
		description = extractDescription(body)
	}
	description, err = p.Text("Description", description)
	if err != nil {
		return err
	}

	var triggers []string
	if fm != nil && len(fm.Triggers) > 0 {
		triggers = fm.Triggers
	} else {
		triggers = []string{"/" + skillName}
	}

	invocable := true
	if fm != nil {
		invocable = fm.Invocable
	}

	argumentHint := ""
	if fm != nil {
		argumentHint = fm.ArgumentHint
	}

	// 6. Build manifest
	fullName := manifest.FormatFullName(scope, skillName)

	m := manifest.Scaffold(manifest.TypeSkill, scope, skillName)
	m.Version = version
	m.Description = description
	m.Visibility = opts.defaultVisibility
	if m.Skill == nil {
		m.Skill = &manifest.SkillSpec{}
	}
	m.Skill.Entry = "SKILL.md"
	m.Skill.UserInvocable = &invocable

	// 7. Build SKILL.md frontmatter
	newFM := &manifest.SkillFrontmatter{
		Name:         skillName,
		Description:  description,
		Triggers:     triggers,
		Invocable:    invocable,
		ArgumentHint: argumentHint,
	}

	// 8. Validate manifest
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		return output.ErrUsageHint("validation failed: "+errs[0], "Fix the issue and try again")
	}

	// 9. Stage
	stg, err := staging.New("ctx-publish-")
	if err != nil {
		return err
	}
	defer stg.Rollback() // cleanup if anything fails

	manifestData, err := manifest.Marshal(m)
	if err != nil {
		return err
	}
	if err := stg.WriteFile("ctx.yaml", manifestData, 0o644); err != nil {
		return fmt.Errorf("stage ctx.yaml: %w", err)
	}

	skillContent, err := manifest.RenderSkillMD(newFM, body)
	if err != nil {
		return err
	}
	if err := stg.WriteFile("SKILL.md", skillContent, 0o644); err != nil {
		return fmt.Errorf("stage SKILL.md: %w", err)
	}

	// 10. Preview
	w.Header("Package Preview")
	w.Table([][]string{
		{"Name:", fullName},
		{"Version:", version},
		{"Type:", "skill"},
		{"Description:", truncate(description, 60)},
		{"Visibility:", opts.defaultVisibility},
		{"Triggers:", strings.Join(triggers, ", ")},
	})
	fmt.Fprintln(os.Stderr)

	// 11. Dry-run: show preview and exit without publishing.
	if opts.dryRun {
		return w.OK(map[string]string{
			"full_name": fullName,
			"version":   version,
			"file":      filePath,
		}, output.WithSummary(fmt.Sprintf("Would publish %s@%s", fullName, version)))
	}

	// 12. Confirm
	confirmed, err := p.Confirm("Publish to registry?", true)
	if err != nil {
		return err
	}
	if !confirmed {
		w.Info("Cancelled.")
		return nil
	}

	// 13. Create archive and publish (SKILL.md content goes to registry)
	w.Info("Publishing %s@%s...", fullName, version)
	archive, err := stg.TarGz()
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { _ = archive.Close() }()

	reg := registry.New(cfg.RegistryURL(), token)

	// Auto-upgrade private→target visibility if needed.
	if err := maybeUpgradeVisibility(cmd.Context(), reg, w, fullName, opts.defaultVisibility, opts.skipConfirm); err != nil {
		if errors.Is(err, errUpgradeCancelled) {
			w.Info("Cancelled.")
			return nil
		}
		return err
	}

	result, err := reg.Publish(cmd.Context(), manifestData, archive, nil)
	if err != nil {
		return err
	}

	// 13. Commit staging to skills dir
	dest := filepath.Join(config.SkillsDir(), scope, skillName)
	if commitErr := stg.Commit(dest); commitErr != nil {
		// Publish succeeded but local commit failed — warn but don't error
		w.Warn("Published to registry, but failed to save locally: %v", commitErr)
		w.Info("Package is available via: ctx install %s", fullName)
		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Published %s@%s (%s)", result.FullName, result.Version, opts.defaultVisibility)),
		)
	}
	w.Success("Saved to %s", dest)

	// 14. Link back to original location
	absFilePath, _ := filepath.Abs(filePath)
	skillMDPath := filepath.Join(dest, "SKILL.md")

	linkBack, err := p.Confirm("Link "+absFilePath+" → "+skillMDPath+"?", true)
	if err != nil {
		return err
	}
	if linkBack {
		if linkErr := linkToOriginal(absFilePath, skillMDPath, fullName); linkErr != nil {
			w.Warn("Could not create link: %v", linkErr)
		} else {
			w.Success("Linked: %s → %s", absFilePath, skillMDPath)
		}
	}

	// 15. Output
	packageURL := cfg.PackageWebURL(result.FullName)
	return w.OK(result,
		output.WithSummary(fmt.Sprintf("Published %s@%s (%s)", result.FullName, result.Version, opts.defaultVisibility)),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "view", Command: packageURL, Description: "View on getctx.org"},
			output.Breadcrumb{Action: "install", Command: "ctx install " + result.FullName, Description: "Install on another device"},
			output.Breadcrumb{Action: "update", Command: "ctx publish " + dest, Description: "Publish updates"},
			output.Breadcrumb{Action: "bump", Command: "ctx publish " + dest + " --bump patch", Description: "Bump version and publish"},
		),
	)
}

