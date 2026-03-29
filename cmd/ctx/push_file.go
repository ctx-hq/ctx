package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/pushstate"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

var flagBump string

func init() {
	pushCmd.Flags().StringVar(&flagBump, "bump", "", "Version bump strategy (patch, minor, major)")
	publishCmd.Flags().StringVar(&flagBump, "bump", "", "Version bump strategy (patch, minor, major)")
}

// singleFileOpts configures push vs publish behavior for single-file skills.
type singleFileOpts struct {
	defaultVisibility string // "private" for push, "public" for publish
	mutable           bool   // true for push
	versionBump       string // "patch"/"minor"/"major"/""
	skipConfirm       bool   // --yes flag
	dryRun            bool   // --dry-run flag
}

// pushSingleFile handles `ctx push <file.md>` and `ctx publish <file.md>`.
func pushSingleFile(cmd *cobra.Command, filePath string, w *output.Writer, opts singleFileOpts) error {
	// 1. Auth check
	token := getToken()
	if token == "" {
		return output.ErrAuth("not logged in — run 'ctx login' first")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Username == "" {
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
		output.Warn("Could not parse frontmatter: %v (treating as plain content)", parseErr)
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
	scope := cfg.Username
	version := resolveBaseVersion(scope, skillName)
	if opts.versionBump != "" {
		bumped, bumpErr := manifest.BumpVersion(version, opts.versionBump)
		if bumpErr != nil {
			return bumpErr
		}
		version = bumped
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

	keywords := triggers

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
	m.Keywords = keywords
	m.Visibility = opts.defaultVisibility
	m.Mutable = opts.mutable
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
	stg, err := staging.New("ctx-push-")
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
	output.Header("Package Preview")
	output.Table([][]string{
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
		}, output.WithSummary(fmt.Sprintf("Would push %s@%s", fullName, version)))
	}

	// 12. Confirm
	confirmed, err := p.Confirm("Publish to registry?", true)
	if err != nil {
		return err
	}
	if !confirmed {
		output.Info("Cancelled.")
		return nil
	}

	// 13. Create archive and publish (SKILL.md content goes to registry)
	output.Info("Publishing %s@%s...", fullName, version)
	archive, err := stg.TarGz()
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { _ = archive.Close() }()

	reg := registry.New(cfg.RegistryURL(), token)
	result, err := reg.Publish(cmd.Context(), manifestData, archive)
	if err != nil {
		return err
	}

	// 13. Commit staging to skills dir
	dest := filepath.Join(config.SkillsDir(), scope, skillName)
	if commitErr := stg.Commit(dest); commitErr != nil {
		// Publish succeeded but local commit failed — warn but don't error
		output.Warn("Published to registry, but failed to save locally: %v", commitErr)
		output.Info("Package is available via: ctx install %s", fullName)
		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Published %s@%s (%s)", result.FullName, result.Version, opts.defaultVisibility)),
		)
	}
	output.Success("Saved to %s", dest)

	// Record push state.
	if pst, loadErr := pushstate.Load(); loadErr == nil {
		if h, hErr := pushstate.HashDir(dest); hErr == nil {
			pst.RecordPush(fullName, h, result.Version, dest)
			_ = pst.Save()
		}
	}

	// 14. Link back to original location
	absFilePath, _ := filepath.Abs(filePath)
	skillMDPath := filepath.Join(dest, "SKILL.md")

	linkBack, err := p.Confirm("Link "+absFilePath+" → "+skillMDPath+"?", true)
	if err != nil {
		return err
	}
	if linkBack {
		if linkErr := linkToOriginal(absFilePath, skillMDPath, fullName); linkErr != nil {
			output.Warn("Could not create link: %v", linkErr)
		} else {
			output.Success("Linked: %s → %s", absFilePath, skillMDPath)
		}
	}

	// 15. Output
	return w.OK(result,
		output.WithSummary(fmt.Sprintf("Published %s@%s (%s)", result.FullName, result.Version, opts.defaultVisibility)),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "install", Command: "ctx install " + result.FullName, Description: "Install on another device"},
			output.Breadcrumb{Action: "update", Command: "ctx push " + dest, Description: "Push updates from local dir"},
			output.Breadcrumb{Action: "bump", Command: "ctx push " + dest + " --bump patch", Description: "Bump version and push"},
		),
	)
}

