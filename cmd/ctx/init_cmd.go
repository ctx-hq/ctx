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

// initMeta holds the parsed/prompted metadata for a package.
type initMeta struct {
	// Common
	name        string
	description string
	version     string
	pkgType     manifest.PackageType

	// Skill
	triggers  []string
	invocable bool
	argHint   string
	body      string // SKILL.md body content

	// CLI
	binary        string
	verify        string
	installMethod string // brew, npm, pip, gem, cargo, script, binary
	installPkg    string // the package identifier for the chosen method
	authHint      string
	bundlesSkill  bool

	// MCP
	transport string
	command   string
	mcpArgs   []string
	mcpURL    string
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
		pkgTypePreview := meta.pkgType
		if pkgTypePreview == "" {
			pkgTypePreview = manifest.TypeSkill
		}
		output.Header("Package Preview")
		previewRows := [][]string{
			{"Name:", fullName},
			{"Version:", meta.version},
			{"Type:", string(pkgTypePreview)},
			{"Description:", truncate(meta.description, 60)},
		}
		if meta.pkgType == manifest.TypeCLI {
			previewRows = append(previewRows, []string{"Binary:", meta.binary})
			previewRows = append(previewRows, []string{"Install:", meta.installMethod + ": " + meta.installPkg})
			if meta.authHint != "" {
				previewRows = append(previewRows, []string{"Auth:", truncate(meta.authHint, 50)})
			}
		}
		if meta.pkgType == manifest.TypeMCP {
			previewRows = append(previewRows, []string{"Transport:", meta.transport})
			if meta.command != "" {
				previewRows = append(previewRows, []string{"Command:", meta.command})
			}
			if meta.mcpURL != "" {
				previewRows = append(previewRows, []string{"URL:", meta.mcpURL})
			}
		}
		previewRows = append(previewRows, []string{"Target:", dest})
		output.Table(previewRows)
		fmt.Fprintln(os.Stderr) // blank line after table

		confirmed, err := p.Confirm("Create package?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			output.Info("Cancelled.")
			return nil
		}

		// 10. Build manifest based on package type
		pkgType := meta.pkgType
		if pkgType == "" {
			pkgType = manifest.TypeSkill
		}
		m := manifest.Scaffold(pkgType, scope, skillName)
		m.Version = meta.version
		m.Description = meta.description

		switch pkgType {
		case manifest.TypeSkill:
			m.Keywords = meta.triggers
			if m.Skill == nil {
				m.Skill = &manifest.SkillSpec{}
			}
			m.Skill.Entry = "SKILL.md"
			m.Skill.UserInvocable = &meta.invocable

		case manifest.TypeCLI:
			m.CLI = &manifest.CLISpec{
				Binary: meta.binary,
				Verify: meta.verify,
				Auth:   meta.authHint,
			}
			m.Install = buildInstallSpec(meta.installMethod, meta.installPkg)
			if meta.bundlesSkill {
				if m.Skill == nil {
					m.Skill = &manifest.SkillSpec{}
				}
				m.Skill.Entry = "SKILL.md"
				m.Skill.Origin = "native"
			}

		case manifest.TypeMCP:
			m.MCP = &manifest.MCPSpec{
				Transport: meta.transport,
				Command:   meta.command,
				Args:      meta.mcpArgs,
				URL:       meta.mcpURL,
			}
		}

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			return output.ErrUsageHint("validation failed: "+errs[0], "Fix the issue and try again")
		}

		manifestData, err := manifest.Marshal(m)
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

		// Overwrite ctx.yaml with normalized version
		if err := stg.WriteFile("ctx.yaml", manifestData, 0o644); err != nil {
			return fmt.Errorf("stage ctx.yaml: %w", err)
		}

		// Generate SKILL.md for skill packages or CLI packages that bundle a skill
		if pkgType == manifest.TypeSkill || (pkgType == manifest.TypeCLI && meta.bundlesSkill) {
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
			if err := stg.WriteFile("SKILL.md", skillContent, 0o644); err != nil {
				return fmt.Errorf("stage SKILL.md: %w", err)
			}
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

		// 13. Link to all detected agents (if package has a skill component)
		if pkgType == manifest.TypeSkill || (pkgType == manifest.TypeCLI && meta.bundlesSkill) {
			if _, linkErr := installer.LinkSkillToAgents(dest, skillName, fullName, ""); linkErr != nil {
				output.Warn("Agent linking: %v", linkErr)
			}
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
			// pkgType left empty — will be prompted in promptMetadata
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
		pkgType:   manifest.TypeSkill, // file/dir mode defaults to skill
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
			pkgType:     m.Type,
		}
		if m.Keywords != nil {
			meta.triggers = m.Keywords
		}
		if m.Skill != nil && m.Skill.UserInvocable != nil {
			meta.invocable = *m.Skill.UserInvocable
		}

		// Preserve CLI fields from existing ctx.yaml
		if m.CLI != nil {
			meta.binary = m.CLI.Binary
			meta.verify = m.CLI.Verify
			meta.authHint = m.CLI.Auth
		}
		if m.Install != nil {
			switch {
			case m.Install.Brew != "":
				meta.installMethod = "brew"
				meta.installPkg = m.Install.Brew
			case m.Install.Npm != "":
				meta.installMethod = "npm"
				meta.installPkg = m.Install.Npm
			case m.Install.Pip != "":
				meta.installMethod = "pip"
				meta.installPkg = m.Install.Pip
			case m.Install.Gem != "":
				meta.installMethod = "gem"
				meta.installPkg = m.Install.Gem
			case m.Install.Cargo != "":
				meta.installMethod = "cargo"
				meta.installPkg = m.Install.Cargo
			case m.Install.Script != "":
				meta.installMethod = "script"
				meta.installPkg = m.Install.Script
			case m.Install.Source != "":
				meta.installMethod = "binary"
				meta.installPkg = m.Install.Source
			}
		}
		if m.Skill != nil && m.Type == manifest.TypeCLI {
			meta.bundlesSkill = true
		}

		// Preserve MCP fields from existing ctx.yaml
		if m.MCP != nil {
			meta.transport = m.MCP.Transport
			meta.command = m.MCP.Command
			meta.mcpArgs = m.MCP.Args
			meta.mcpURL = m.MCP.URL
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

	// Package type selection (only for from-scratch mode where type isn't pre-set)
	if meta.pkgType == "" {
		typeIdx, err := p.Select("Package type", []string{"skill", "cli", "mcp"}, 0)
		if err != nil {
			return meta, err
		}
		types := []manifest.PackageType{manifest.TypeSkill, manifest.TypeCLI, manifest.TypeMCP}
		meta.pkgType = types[typeIdx]
	}

	// Type-specific prompts
	switch meta.pkgType {
	case manifest.TypeCLI:
		meta, err = promptCLIMeta(p, meta)
	case manifest.TypeMCP:
		meta, err = promptMCPMeta(p, meta)
	}
	if err != nil {
		return meta, err
	}

	return meta, nil
}

// installMethods defines the available CLI install methods.
var installMethods = []struct {
	key   string
	label string
}{
	{"brew", "brew (Homebrew formula/tap)"},
	{"npm", "npm (npm package)"},
	{"pip", "pip (Python package)"},
	{"gem", "gem (Ruby gem)"},
	{"cargo", "cargo (Rust crate)"},
	{"script", "script (curl | sh URL)"},
	{"binary", "binary (direct download URL)"},
}

// promptCLIMeta prompts for CLI-specific metadata.
func promptCLIMeta(p prompt.Prompter, meta initMeta) (initMeta, error) {
	var err error

	if meta.binary == "" {
		meta.binary = meta.name
	}
	meta.binary, err = p.Text("Binary name", meta.binary)
	if err != nil {
		return meta, err
	}

	defaultVerify := meta.binary + " --version"
	if meta.verify == "" {
		meta.verify = defaultVerify
	}
	meta.verify, err = p.Text("Verify command", meta.verify)
	if err != nil {
		return meta, err
	}

	// Install method selection
	labels := make([]string, len(installMethods))
	for i, m := range installMethods {
		labels[i] = m.label
	}
	methodIdx, err := p.Select("Install method", labels, 0)
	if err != nil {
		return meta, err
	}
	meta.installMethod = installMethods[methodIdx].key

	// Prompt for the package identifier based on chosen method
	var pkgLabel string
	switch meta.installMethod {
	case "brew":
		pkgLabel = "Brew formula/tap"
	case "npm":
		pkgLabel = "npm package"
	case "pip":
		pkgLabel = "pip package"
	case "gem":
		pkgLabel = "gem name"
	case "cargo":
		pkgLabel = "cargo crate"
	case "script":
		pkgLabel = "Script URL (https://)"
	case "binary":
		pkgLabel = "Binary download URL (https://)"
	}
	meta.installPkg, err = p.Text(pkgLabel, meta.installPkg)
	if err != nil {
		return meta, err
	}

	// Auth hint
	meta.authHint, err = p.Text("Auth setup hint (leave empty if not needed)", meta.authHint)
	if err != nil {
		return meta, err
	}

	// Bundles a skill?
	meta.bundlesSkill, err = p.Confirm("Bundles a SKILL.md?", false)
	if err != nil {
		return meta, err
	}

	return meta, nil
}

// buildInstallSpec creates a manifest InstallSpec from the chosen method and package.
func buildInstallSpec(method, pkg string) *manifest.InstallSpec {
	spec := &manifest.InstallSpec{}
	switch method {
	case "brew":
		spec.Brew = pkg
	case "npm":
		spec.Npm = pkg
	case "pip":
		spec.Pip = pkg
	case "gem":
		spec.Gem = pkg
	case "cargo":
		spec.Cargo = pkg
	case "script":
		spec.Script = pkg
	case "binary":
		spec.Source = pkg
	}
	return spec
}

// promptMCPMeta prompts for MCP-specific metadata.
func promptMCPMeta(p prompt.Prompter, meta initMeta) (initMeta, error) {
	var err error

	transports := []string{"stdio", "sse", "http", "streamable-http"}
	transportIdx, err := p.Select("Transport", transports, 0)
	if err != nil {
		return meta, err
	}
	meta.transport = transports[transportIdx]

	if meta.transport == "stdio" {
		meta.command, err = p.Text("Command", meta.command)
		if err != nil {
			return meta, err
		}
		argsStr, err := p.Text("Arguments (comma-separated)", "")
		if err != nil {
			return meta, err
		}
		if argsStr != "" {
			for _, arg := range strings.Split(argsStr, ",") {
				arg = strings.TrimSpace(arg)
				if arg != "" {
					meta.mcpArgs = append(meta.mcpArgs, arg)
				}
			}
		}
	} else {
		meta.mcpURL, err = p.Text("Server URL (https://...)", meta.mcpURL)
		if err != nil {
			return meta, err
		}
		if meta.mcpURL != "" && !strings.HasPrefix(meta.mcpURL, "https://") && !strings.HasPrefix(meta.mcpURL, "http://") {
			return meta, fmt.Errorf("server URL must start with https:// or http://")
		}
	}

	return meta, nil
}

