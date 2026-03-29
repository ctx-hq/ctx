package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

// initMode describes how ctx init was invoked.
type initMode int

const (
	initFromScratch    initMode = iota // ctx init (no args)
	initFromFile                       // ctx init gc.md
	initFromDirectory                  // ctx init ./my-skill/
)

// initInput captures the detected input for ctx init.
type initInput struct {
	mode       initMode
	sourcePath string // absolute path to source file (initFromFile)
	sourceDir  string // absolute path to source dir (initFromDirectory)
}

// initMeta holds the parsed/prompted metadata for a skill.
type initMeta struct {
	name        string
	description string
	version     string
	triggers    []string
	invocable   bool
	argHint     string
	body        string // SKILL.md body content
}

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Bring a skill into ctx management",
	Long: `Standardize and manage a skill under ~/.ctx/skills/ (Single Source of Truth).

Supports three input modes:
  1. From scratch       ctx init
  2. From .md file      ctx init gc.md
  3. From directory     ctx init ./my-skill/

All modes normalize the skill into a standard structure (ctx.yaml + SKILL.md),
archive it to ~/.ctx/skills/{scope}/{name}/, and link it to detected agents.

Examples:
  ctx init                     Create a new skill interactively
  ctx init gc.md               Adopt an existing .md file
  ctx init ./my-skill/         Adopt an existing skill directory
  ctx init gc.md -y            Non-interactive with defaults`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// 1. Resolve scope
		scope := resolveScope()

		// 2. Detect input mode
		input, err := detectInitInput(args)
		if err != nil {
			return err
		}

		// 3. Check if source is already managed (symlink to SSOT)
		if input.mode == initFromFile {
			if target, linkErr := os.Readlink(input.sourcePath); linkErr == nil {
				if strings.HasPrefix(target, config.SkillsDir()) {
					output.Info("Already managed: %s → %s", input.sourcePath, target)
					return nil
				}
			}
		}

		// 4. Parse source into metadata
		meta, err := parseInitSource(input)
		if err != nil {
			return err
		}

		// 5. Create prompter
		var p prompt.Prompter
		if flagYes {
			p = prompt.NoopPrompter{}
		} else {
			p = prompt.DefaultPrompter()
		}

		// 6. Interactive metadata prompts
		meta, err = promptMetadata(p, meta)
		if err != nil {
			return err
		}

		// 7. Compute destination
		skillName := slugify(meta.name)
		fullName := manifest.FormatFullName(scope, skillName)
		dest := filepath.Join(config.SkillsDir(), scope, skillName)

		// 8. Check if already exists in SSOT
		if _, statErr := os.Stat(dest); statErr == nil {
			overwrite, confirmErr := p.Confirm(fmt.Sprintf("%s already exists, overwrite?", fullName), false)
			if confirmErr != nil {
				return confirmErr
			}
			if !overwrite {
				output.Info("Cancelled.")
				return nil
			}
		}

		// 9. Preview and confirm
		output.Header("Skill Preview")
		output.Table([][]string{
			{"Name:", fullName},
			{"Version:", meta.version},
			{"Type:", "skill"},
			{"Description:", truncate(meta.description, 60)},
			{"Target:", dest},
		})
		fmt.Fprintln(os.Stderr) // blank line after table

		confirmed, err := p.Confirm("Create skill?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			output.Info("Cancelled.")
			return nil
		}

		// 10. Build manifest and SKILL.md
		m := manifest.Scaffold(manifest.TypeSkill, scope, skillName)
		m.Version = meta.version
		m.Description = meta.description
		m.Keywords = meta.triggers
		if m.Skill == nil {
			m.Skill = &manifest.SkillSpec{}
		}
		m.Skill.Entry = "SKILL.md"
		m.Skill.UserInvocable = &meta.invocable

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			return output.ErrUsageHint("validation failed: "+errs[0], "Fix the issue and try again")
		}

		manifestData, err := manifest.Marshal(m)
		if err != nil {
			return err
		}

		fm := &manifest.SkillFrontmatter{
			Name:         skillName,
			Description:  meta.description,
			Triggers:     meta.triggers,
			Invocable:    meta.invocable,
			ArgumentHint: meta.argHint,
		}
		skillContent, err := manifest.RenderSkillMD(fm, meta.body)
		if err != nil {
			return err
		}

		// 11. Stage and commit atomically
		stg, err := staging.New("ctx-init-")
		if err != nil {
			return err
		}
		defer stg.Rollback()

		// For directory mode, copy all source files first to preserve
		// scripts/, assets/, references/ and other skill dependencies.
		if input.mode == initFromDirectory {
			if err := stg.CopyFrom(input.sourceDir); err != nil {
				return fmt.Errorf("copy source directory: %w", err)
			}
		}

		// Overwrite ctx.yaml and SKILL.md with normalized versions
		if err := stg.WriteFile("ctx.yaml", manifestData, 0o644); err != nil {
			return fmt.Errorf("stage ctx.yaml: %w", err)
		}
		if err := stg.WriteFile("SKILL.md", skillContent, 0o644); err != nil {
			return fmt.Errorf("stage SKILL.md: %w", err)
		}

		if err := stg.Commit(dest); err != nil {
			return fmt.Errorf("commit to %s: %w", dest, err)
		}

		output.Success("Created %s → %s", fullName, dest)

		// 12. Symlink back to original location (single-file mode)
		if input.mode == initFromFile {
			skillMDPath := filepath.Join(dest, "SKILL.md")
			if linkErr := linkToOriginal(input.sourcePath, skillMDPath, fullName); linkErr != nil {
				output.Warn("Could not link back: %v", linkErr)
			} else {
				output.Success("Linked: %s → %s", input.sourcePath, skillMDPath)
			}
		}

		// 13. Link to all detected agents
		if linkErr := installer.LinkSkillToAgents(dest, skillName, fullName, ""); linkErr != nil {
			output.Warn("Agent linking: %v", linkErr)
		}

		// 14. Output
		return w.OK(
			map[string]string{"name": fullName, "version": meta.version, "path": dest},
			output.WithSummary(fmt.Sprintf("Created %s (%s)", fullName, meta.version)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "edit", Command: "edit " + filepath.Join(dest, "SKILL.md"), Description: "Edit skill content"},
				output.Breadcrumb{Action: "publish", Command: "ctx push " + fullName, Description: "Publish to registry"},
			),
		)
	},
}

// resolveScope returns the user's scope from config, falling back to "local".
func resolveScope() string {
	cfg, err := config.Load()
	if err != nil {
		return "local"
	}
	if cfg.Username != "" {
		return cfg.Username
	}
	return "local"
}

// detectInitInput determines the input mode from command args.
func detectInitInput(args []string) (initInput, error) {
	if len(args) == 0 {
		return initInput{mode: initFromScratch}, nil
	}

	path := args[0]
	absPath, err := filepath.Abs(path)
	if err != nil {
		return initInput{}, fmt.Errorf("resolve path: %w", err)
	}

	info, statErr := os.Lstat(absPath)

	// File: must be .md
	if statErr == nil && !info.IsDir() {
		if !strings.HasSuffix(strings.ToLower(absPath), ".md") {
			return initInput{}, output.ErrUsageHint(
				"single-file init supports .md files only",
				"Provide a .md file or a directory",
			)
		}
		return initInput{mode: initFromFile, sourcePath: absPath}, nil
	}

	// Directory
	if statErr == nil && info.IsDir() {
		return initInput{mode: initFromDirectory, sourceDir: absPath}, nil
	}

	// Path doesn't exist — report error so the user knows
	return initInput{}, fmt.Errorf("path not found: %s", path)
}

// parseInitSource extracts metadata from the input source.
func parseInitSource(input initInput) (initMeta, error) {
	switch input.mode {
	case initFromScratch:
		cwd, err := os.Getwd()
		if err != nil {
			return initMeta{}, fmt.Errorf("get working directory: %w", err)
		}
		return initMeta{
			name:      filepath.Base(cwd),
			version:   "0.1.0",
			invocable: true,
		}, nil

	case initFromFile:
		return parseFileSource(input.sourcePath)

	case initFromDirectory:
		return parseDirSource(input.sourceDir)

	default:
		return initMeta{}, fmt.Errorf("unknown init mode")
	}
}

// metaFromFrontmatter builds an initMeta from parsed frontmatter and body,
// using fallbackName when frontmatter fields are missing.
func metaFromFrontmatter(fm *manifest.SkillFrontmatter, body, fallbackName string) initMeta {
	meta := initMeta{
		name:      slugify(fallbackName),
		version:   "0.1.0",
		invocable: true,
		body:      body,
	}

	if fm != nil {
		if fm.Name != "" {
			meta.name = fm.Name
		}
		if fm.Description != "" {
			meta.description = fm.Description
		}
		if len(fm.Triggers) > 0 {
			meta.triggers = fm.Triggers
		}
		meta.invocable = fm.Invocable
		meta.argHint = fm.ArgumentHint
	}

	if meta.description == "" {
		meta.description = extractDescription(body)
	}
	if len(meta.triggers) == 0 {
		meta.triggers = []string{"/" + slugify(meta.name)}
	}

	return meta
}

// parseFileSource reads a .md file and extracts metadata.
func parseFileSource(filePath string) (initMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return initMeta{}, fmt.Errorf("read file: %w", err)
	}
	if len(data) == 0 {
		return initMeta{}, output.ErrUsageHint("file is empty", "Add skill content to "+filePath)
	}

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	fm, body, parseErr := manifest.ParseSkillMD(strings.NewReader(string(data)))
	if parseErr != nil {
		// Malformed frontmatter YAML — treat file content as body with no frontmatter
		return metaFromFrontmatter(nil, string(data), baseName), nil
	}

	return metaFromFrontmatter(fm, body, baseName), nil
}

// parseDirSource reads an existing directory and extracts metadata.
func parseDirSource(dirPath string) (initMeta, error) {
	dirName := filepath.Base(dirPath)

	// Try ctx.yaml first
	if m, err := manifest.LoadFromDir(dirPath); err == nil {
		_, shortName := manifest.ParseFullName(m.Name)
		meta := initMeta{
			name:        shortName,
			description: m.Description,
			version:     m.Version,
		}
		if m.Keywords != nil {
			meta.triggers = m.Keywords
		}
		if m.Skill != nil && m.Skill.UserInvocable != nil {
			meta.invocable = *m.Skill.UserInvocable
		}

		// Read SKILL.md frontmatter + body to preserve fields like argument-hint
		skillPath := filepath.Join(dirPath, "SKILL.md")
		if fm, body, readErr := readAndParseSkillMD(skillPath); readErr == nil {
			meta.body = body
			if fm != nil {
				meta.argHint = fm.ArgumentHint
			}
		}

		if len(meta.triggers) == 0 {
			meta.triggers = []string{"/" + slugify(meta.name)}
		}
		return meta, nil
	}

	// Try SKILL.md
	skillPath := filepath.Join(dirPath, "SKILL.md")
	fm, body, err := readAndParseSkillMD(skillPath)
	if err != nil {
		// Nothing found — scaffold from directory name
		return metaFromFrontmatter(nil, "", dirName), nil
	}

	return metaFromFrontmatter(fm, body, dirName), nil
}

// readAndParseSkillMD opens a SKILL.md file and returns frontmatter and body.
func readAndParseSkillMD(path string) (*manifest.SkillFrontmatter, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()
	fm, body, parseErr := manifest.ParseSkillMD(f)
	if parseErr != nil {
		return nil, body, parseErr
	}
	return fm, body, nil
}

// promptMetadata interactively fills in missing metadata fields.
func promptMetadata(p prompt.Prompter, meta initMeta) (initMeta, error) {
	var err error

	meta.name, err = p.Text("Package name", meta.name)
	if err != nil {
		return meta, err
	}
	meta.name = slugify(meta.name)

	meta.description, err = p.Text("Description", meta.description)
	if err != nil {
		return meta, err
	}

	meta.version, err = p.Text("Version", meta.version)
	if err != nil {
		return meta, err
	}

	return meta, nil
}

