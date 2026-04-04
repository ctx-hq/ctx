package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/initdetect"
	"github.com/ctx-hq/ctx/internal/license"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/tui/inline"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	sourceDir   string // original directory path (initFromDirectory mode)

	// Skill (all types require a skill component)
	triggers   []string
	invocable  bool
	argHint    string
	body       string // SKILL.md body content
	skillEntry string // skill.entry path (e.g., "SKILL.md", "skills/fizzy/SKILL.md")

	// CLI
	binary        string
	verify        string
	installMethod string // brew, npm, pip, gem, cargo, script, binary
	installPkg    string // the package identifier for the chosen method
	authHint      string

	// MCP
	transport string
	command   string
	mcpArgs   []string
	mcpURL    string

	// Auto-detected metadata
	author     string
	license    string
	repository string
}

var (
	flagInitType   string // --type skill|mcp|cli
	flagInitName   string // --name @scope/name
	flagInitFrom   string // --from npm:pkg|github:owner/repo|docker:image|/path
	flagInitImport bool   // --import: auto-detect and import existing skill repo
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a ctx package (skill, CLI, or MCP)",
	Long: `Initialize a ctx package in the current directory (like npm init).

Generates ctx.yaml and SKILL.md in the project directory.
No files are copied elsewhere — authoring happens locally.

Supports four input modes:
  1. From scratch       ctx init
  2. From .md file      ctx init gc.md
  3. From directory     ctx init ./my-skill/
  4. From upstream      ctx init --from npm:@playwright/mcp

Examples:
  ctx init                                    Create a new package interactively
  ctx init gc.md                              Adopt an existing .md file as a skill
  ctx init ./my-skill/                        Initialize from an existing directory
  ctx init --from npm:@playwright/mcp         Auto-detect from npm package
  ctx init --from github:github/github-mcp-server  Auto-detect from GitHub repo
  ctx init --from docker:ghcr.io/org/image    Auto-detect from Docker image`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// --import mode: auto-detect and import existing skill repo
		if flagInitImport {
			return runInitImport(cmd, w)
		}

		// --from mode: auto-detect from upstream source
		if flagInitFrom != "" {
			return runInitFrom(cmd, w)
		}

		// 1. Resolve scope
		scope := resolveScope()

		// 2. Detect input mode
		input, err := detectInitInput(args)
		if err != nil {
			return err
		}

		// 3. Determine output directory (where ctx.yaml will be written)
		outDir, err := resolveOutputDir(input)
		if err != nil {
			return err
		}

		// 4. Parse source into metadata
		meta, err := parseInitSource(input)
		if err != nil {
			return err
		}

		// Apply --type flag (overrides detected/default type)
		if flagInitType != "" {
			pt := manifest.PackageType(flagInitType)
			if !pt.Valid() {
				return output.ErrUsage(fmt.Sprintf("--type must be skill, mcp, or cli (got %q)", flagInitType))
			}
			meta.pkgType = pt
		}

		// Apply --name flag (overrides scope + name + default description)
		if flagInitName != "" {
			s, n := manifest.ParseFullName(flagInitName)
			if s != "" {
				scope = s
			}
			if n != "" {
				meta.name = n
				// Update default description to match the actual package name
				if meta.description == "" || strings.HasPrefix(meta.description, "A ") {
					meta.description = fmt.Sprintf("A %s package", n)
				}
			}
		}

		// 5. Create prompter
		var p prompt.Prompter
		if flagYes {
			p = prompt.NoopPrompter{}
		} else if term.IsTerminal(int(os.Stdin.Fd())) {
			p = inline.NewBubblePrompter()
		} else {
			p = prompt.NoopPrompter{}
		}

		// 6. Interactive metadata prompts
		if input.mode == initFromDirectory {
			meta.sourceDir = input.sourceDir
		}
		meta, err = promptMetadata(p, meta)
		if err != nil {
			return err
		}

		// 7. Check if ctx.yaml already exists
		skillName := slugify(meta.name)
		fullName := manifest.FormatFullName(scope, skillName)

		ctxYamlPath := filepath.Join(outDir, "ctx.yaml")
		if _, statErr := os.Stat(ctxYamlPath); statErr == nil {
			overwrite, confirmErr := p.Confirm("ctx.yaml already exists, overwrite?", false)
			if confirmErr != nil {
				return confirmErr
			}
			if !overwrite {
				w.Info("Cancelled.")
				return nil
			}
		}

		// 8. Preview and confirm
		pkgTypePreview := meta.pkgType
		if pkgTypePreview == "" {
			pkgTypePreview = manifest.TypeSkill
		}
		w.Header("Package Preview")
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
		if meta.author != "" {
			previewRows = append(previewRows, []string{"Author:", meta.author})
		}
		if meta.license != "" {
			previewRows = append(previewRows, []string{"License:", meta.license})
		}
		if meta.repository != "" {
			previewRows = append(previewRows, []string{"Repository:", meta.repository})
		}
		previewRows = append(previewRows, []string{"Directory:", outDir})
		w.Table(previewRows)
		fmt.Fprintln(os.Stderr) // blank line after table

		confirmed, err := p.Confirm("Create package?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			w.Info("Cancelled.")
			return nil
		}

		// 9. Build manifest based on package type
		pkgType := meta.pkgType
		if pkgType == "" {
			pkgType = manifest.TypeSkill
		}
		m := manifest.Scaffold(pkgType, scope, skillName)
		m.Version = meta.version
		m.Description = meta.description
		m.Author = meta.author
		m.License = meta.license
		m.Repository = meta.repository

		// Determine skill entry path
		var skillEntry string
		if m.Skill != nil {
			skillEntry = m.Skill.Entry
		}
		if meta.skillEntry != "" {
			skillEntry = meta.skillEntry
		}

		switch pkgType {
		case manifest.TypeSkill:
			m.Skill.Entry = skillEntry
			m.Skill.UserInvocable = &meta.invocable

		case manifest.TypeCLI:
			m.CLI = &manifest.CLISpec{
				Binary: meta.binary,
				Verify: meta.verify,
				Auth:   meta.authHint,
			}
			if meta.installMethod != "" && meta.installPkg != "" {
				m.Install = buildInstallSpec(meta.installMethod, meta.installPkg)
			}
			m.Skill.Entry = skillEntry
			m.Skill.Origin = "native"

		case manifest.TypeMCP:
			m.MCP = &manifest.MCPSpec{
				Transport: meta.transport,
				Command:   meta.command,
				Args:      meta.mcpArgs,
				URL:       meta.mcpURL,
			}
			// MCP servers are self-describing; SKILL.md is optional
			if skillEntry != "" {
				if m.Skill == nil {
					m.Skill = &manifest.SkillSpec{}
				}
				m.Skill.Entry = skillEntry
				m.Skill.Origin = "native"
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

		// 10. Write ctx.yaml to output directory
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		if err := os.WriteFile(ctxYamlPath, manifestData, 0o644); err != nil {
			return fmt.Errorf("write ctx.yaml: %w", err)
		}

		// 11. Generate SKILL.md if it doesn't already exist at the entry path
		if skillEntry != "" {
			skillAbsPath := filepath.Join(outDir, skillEntry)
			if _, statErr := os.Stat(skillAbsPath); os.IsNotExist(statErr) {
				// Create parent directories for nested skill paths (e.g., skills/fizzy/)
				if err := os.MkdirAll(filepath.Dir(skillAbsPath), 0o755); err != nil {
					return fmt.Errorf("create skill directory: %w", err)
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
				if err := os.WriteFile(skillAbsPath, skillContent, 0o644); err != nil {
					return fmt.Errorf("write SKILL.md: %w", err)
				}
			}
		}

		w.Success("Created %s in %s", fullName, outDir)

		// 12. Output
		var breadcrumbs []output.Breadcrumb
		if skillEntry != "" {
			breadcrumbs = append(breadcrumbs,
				output.Breadcrumb{Action: "edit", Command: "edit " + filepath.Join(outDir, skillEntry), Description: "Edit skill content"},
			)
		}
		if pkgType == manifest.TypeCLI {
			breadcrumbs = append(breadcrumbs,
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish to registry"},
				output.Breadcrumb{Action: "upload", Command: "ctx artifact upload " + fullName + "@" + meta.version + " --dir dist/", Description: "Upload platform binaries"},
			)
		} else {
			breadcrumbs = append(breadcrumbs,
				output.Breadcrumb{Action: "publish", Command: "ctx publish " + outDir, Description: "Publish to registry"},
			)
		}
		return w.OK(
			map[string]string{"name": fullName, "version": meta.version, "path": outDir},
			output.WithSummary(fmt.Sprintf("Created %s (%s)", fullName, meta.version)),
			output.WithBreadcrumbs(breadcrumbs...),
		)
	},
}

// resolveScope returns the user's scope from config, falling back to "local".
func resolveScope() string {
	if username := resolvedUsername(); username != "" {
		return username
	}
	return "local"
}

// resolveOutputDir determines where ctx.yaml should be written.
func resolveOutputDir(input initInput) (string, error) {
	switch input.mode {
	case initFromScratch:
		dir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		return dir, nil
	case initFromFile:
		return filepath.Dir(input.sourcePath), nil
	case initFromDirectory:
		return input.sourceDir, nil
	default:
		return "", fmt.Errorf("unknown init mode")
	}
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
		dirName := filepath.Base(cwd)

		// Check if SKILL.md exists in cwd or skills/*/ — extract metadata from first found.
		found := findAllSkillMD(cwd)
		if len(found) > 0 {
			skillPath := filepath.Join(cwd, found[0])
			fm, body, readErr := readAndParseSkillMD(skillPath)
			if readErr != nil {
				return initMeta{}, fmt.Errorf("parse SKILL.md: %w", readErr)
			}
			meta := metaFromFrontmatter(fm, body, dirName)
			meta.skillEntry = found[0]
			// SKILL.md exists in both skill and cli types — don't assume type,
			// let the user choose via the type prompt.
			meta.pkgType = ""
			autoDetectMeta(&meta, cwd)
			return meta, nil
		}

		meta := initMeta{
			name:        slugify(dirName),
			description: fmt.Sprintf("A %s package", dirName),
			version:     "0.1.0",
			invocable:   true,
			// pkgType left empty — will be prompted in promptMetadata
		}
		autoDetectMeta(&meta, cwd)
		return meta, nil

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
	var meta initMeta
	if parseErr != nil {
		meta = metaFromFrontmatter(nil, string(data), baseName)
	} else {
		meta = metaFromFrontmatter(fm, body, baseName)
	}

	// Auto-detect metadata from the file's parent directory
	autoDetectMeta(&meta, filepath.Dir(filePath))
	return meta, nil
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
		// Preserve MCP fields from existing ctx.yaml
		if m.MCP != nil {
			meta.transport = m.MCP.Transport
			meta.command = m.MCP.Command
			meta.mcpArgs = m.MCP.Args
			meta.mcpURL = m.MCP.URL
		}

		// Preserve skill.entry from existing ctx.yaml
		if m.Skill != nil && m.Skill.Entry != "" {
			meta.skillEntry = m.Skill.Entry
		}

		// Read SKILL.md from the declared entry path (not hardcoded root)
		skillSearchPath := filepath.Join(dirPath, "SKILL.md")
		if m.Skill != nil && m.Skill.Entry != "" {
			skillSearchPath = filepath.Join(dirPath, m.Skill.Entry)
		}
		if fm, body, readErr := readAndParseSkillMD(skillSearchPath); readErr == nil {
			meta.body = body
			if fm != nil {
				meta.argHint = fm.ArgumentHint
				if len(fm.Triggers) > 0 {
					meta.triggers = fm.Triggers
				}
			}
		}

		// Preserve existing manifest values, then fill gaps via auto-detect
		meta.author = m.Author
		meta.repository = m.Repository
		meta.license = m.License
		autoDetectMeta(&meta, dirPath)

		if len(meta.triggers) == 0 {
			meta.triggers = []string{"/" + slugify(meta.name)}
		}
		return meta, nil
	}

	// Try to find SKILL.md (root first, then subdirectories)
	found := findAllSkillMD(dirPath)
	if len(found) > 0 {
		// Use first found (root has priority)
		skillPath := filepath.Join(dirPath, found[0])
		fm, body, err := readAndParseSkillMD(skillPath)
		if err == nil {
			meta := metaFromFrontmatter(fm, body, dirName)
			meta.skillEntry = found[0]
			// SKILL.md exists in both skill and cli types — don't assume type,
			// let the user choose via the type prompt.
			meta.pkgType = ""
			autoDetectMeta(&meta, dirPath)
			return meta, nil
		}
	}

	// Nothing found — scaffold from directory name with default triggers,
	// then clear pkgType so the interactive type prompt will trigger.
	meta := metaFromFrontmatter(nil, "", dirName)
	meta.pkgType = ""
	autoDetectMeta(&meta, dirPath)
	return meta, nil
}

// autoDetectMeta fills in author, repository, license, and version from git/filesystem
// if they are not already set.
func autoDetectMeta(meta *initMeta, dir string) {
	if meta.author == "" {
		meta.author = gitutil.Author(dir)
	}
	if meta.repository == "" {
		meta.repository = gitutil.RemoteURL(dir)
	}
	if meta.license == "" {
		if lr := license.Detect(dir); lr.SPDX != "" {
			meta.license = lr.SPDX
		}
	}
	// Use latest git tag as version if still at default
	if meta.version == "0.1.0" {
		if tag := gitutil.LatestTag(dir); tag != "" {
			meta.version = tag
		}
	}
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

	// 1. Type selection FIRST — type determines the entire subsequent flow.
	// Skipped when pre-set (e.g., --type flag, initFromFile auto-detection,
	// or initFromDirectory with existing ctx.yaml).
	if meta.pkgType == "" {
		typeLabels := []string{
			"skill - Reusable AI agent instructions",
			"mcp   - MCP server (stdio/sse/http)",
			"cli   - CLI tool wrapper",
		}
		types := []manifest.PackageType{manifest.TypeSkill, manifest.TypeMCP, manifest.TypeCLI}
		typeIdx, err := p.Select("Package type", typeLabels, 0)
		if err != nil {
			return meta, err
		}
		meta.pkgType = types[typeIdx]
	}

	// 2. Common prompts
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

	// Auto-detected metadata prompts
	meta.author, err = p.Text("Author", meta.author)
	if err != nil {
		return meta, err
	}
	meta.license, err = p.Text("License (SPDX)", meta.license)
	if err != nil {
		return meta, err
	}
	meta.repository, err = p.Text("Repository URL", meta.repository)
	if err != nil {
		return meta, err
	}

	// 3. Type-specific prompts
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

	// Detect existing SKILL.md files if in directory mode and no entry set yet
	if meta.sourceDir != "" && meta.skillEntry == "" {
		found := findAllSkillMD(meta.sourceDir)
		if len(found) == 1 {
			meta.skillEntry = found[0]
		} else if len(found) > 1 {
			// Multiple found — let user pick
			idx, selectErr := p.Select("Multiple SKILL.md found, select one", found, 0)
			if selectErr != nil {
				return meta, selectErr
			}
			meta.skillEntry = found[idx]
		}
		// If none found, skillEntry stays empty → Scaffold default will be used
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

// findAllSkillMD searches a directory for SKILL.md files.
// Returns relative paths, root first, then skills/*/SKILL.md.
// Only checks root and the conventional skills/*/ subdirectory —
// deeper or non-standard paths must be declared via skill.entry in ctx.yaml.
func findAllSkillMD(dirPath string) []string {
	var results []string
	// Check root
	if _, err := os.Stat(filepath.Join(dirPath, "SKILL.md")); err == nil {
		results = append(results, "SKILL.md")
	}
	// Check skills/*/SKILL.md — Glob only returns ErrBadPattern for
	// malformed patterns; this pattern is static so the error is safe to discard.
	matches, _ := filepath.Glob(filepath.Join(dirPath, "skills", "*", "SKILL.md"))
	for _, m := range matches {
		rel, err := filepath.Rel(dirPath, m)
		if err == nil {
			results = append(results, rel)
		}
	}
	return results
}

func init() {
	initCmd.Flags().StringVar(&flagInitType, "type", "", "Package type: skill, mcp, or cli")
	initCmd.Flags().StringVar(&flagInitName, "name", "", "Full package name (@scope/name)")
	initCmd.Flags().StringVar(&flagInitFrom, "from", "", "Auto-detect from upstream (npm:pkg, github:owner/repo, docker:image, /path)")
	initCmd.Flags().BoolVar(&flagInitImport, "import", false, "Auto-detect and import existing skill repo formats into ctx.yaml")
}

// runInitFrom handles the --from flag: auto-detect from upstream source and generate ctx.yaml.
func runInitFrom(cmd *cobra.Command, w *output.Writer) error {
	detector := initdetect.NewDetector()

	kind, key := initdetect.ParseSource(flagInitFrom)
	w.Info("Detecting source: %s (%s)", kind, key)

	result, err := detector.Detect(cmd.Context(), flagInitFrom)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	w.PrintDim("  Type: %s", result.PackageType)
	w.PrintDim("  Name: %s", result.Name)
	w.PrintDim("  Version: %s", result.Version)
	if result.License != "" {
		w.PrintDim("  License: %s", result.License)
	}
	if result.MCP != nil {
		w.PrintDim("  Transport: %s", result.MCP.Transport)
		if len(result.MCP.Transports) > 0 {
			w.PrintDim("  Additional transports: %d", len(result.MCP.Transports))
		}
		if len(result.MCP.Tools) > 0 {
			w.PrintDim("  Tools: %d detected", len(result.MCP.Tools))
		}
	}

	// Generate manifest
	m := initdetect.ToManifest(result)

	// Validate
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		w.Warn("Generated manifest has validation issues:")
		for _, e := range errs {
			w.PrintDim("  - %s", e)
		}
		w.Info("Review and fix the generated ctx.yaml before publishing.")
	}

	// Write ctx.yaml
	outPath := filepath.Join(".", manifest.FileName)
	if _, err := os.Stat(outPath); err == nil {
		if flagYes {
			w.Warn("Overwriting existing ctx.yaml (--yes)")
		} else if term.IsTerminal(int(os.Stdin.Fd())) {
			p := prompt.DefaultPrompter()
			overwrite, confirmErr := p.Confirm("ctx.yaml already exists, overwrite?", false)
			if confirmErr != nil {
				return confirmErr
			}
			if !overwrite {
				w.Info("Cancelled.")
				return nil
			}
		} else {
			return fmt.Errorf("ctx.yaml already exists — remove it first or use a different directory")
		}
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write ctx.yaml: %w", err)
	}

	// Fetch upstream README if available
	readmePath := "README.md"
	if _, statErr := os.Stat(readmePath); os.IsNotExist(statErr) {
		if readme := initdetect.FetchUpstreamREADME(cmd.Context(), result); readme != nil {
			if writeErr := os.WriteFile(readmePath, readme, 0o644); writeErr == nil {
				w.PrintDim("  Fetched upstream README.md (%d bytes)", len(readme))
			}
		}
	}

	w.Info("Generated %s", outPath)
	w.PrintDim("")
	w.PrintDim("  Next steps:")
	if m.Type == manifest.TypeMCP {
		w.PrintDim("    ctx mcp test %s        Verify the MCP server works", m.ShortName())
	}
	w.PrintDim("    Review the generated ctx.yaml")
	w.PrintDim("    ctx publish")

	return w.OK(map[string]interface{}{
		"file":    outPath,
		"name":    m.Name,
		"version": m.Version,
		"type":    string(m.Type),
		"source":  flagInitFrom,
	}, output.WithSummary("Generated "+outPath))
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
		defaultCmd := meta.command
		if defaultCmd == "" {
			defaultCmd = "npx"
		}
		meta.command, err = p.Text("Command", defaultCmd)
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

