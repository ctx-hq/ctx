package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const FileName = "ctx.yaml"

var (
	nameRegex      = regexp.MustCompile(`^@[a-z0-9]([a-z0-9-]*[a-z0-9])?/[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	semverRegex    = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)
	githubRepoRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
)

// LoadFromDir reads and parses ctx.yaml from a directory.
func LoadFromDir(dir string) (*Manifest, error) {
	path := filepath.Join(dir, FileName)
	return LoadFromFile(path)
}

// LoadFromFile reads and parses a ctx.yaml file.
func LoadFromFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Parse(f)
}

// Parse reads a manifest from a reader.
func Parse(r io.Reader) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse ctx.yaml: %w", err)
	}
	return &m, nil
}

// Validate checks a manifest for correctness.
func Validate(m *Manifest) []string {
	var errs []string

	errs = append(errs, validateCore(m)...)
	errs = append(errs, validateSkillSection(m)...)

	switch m.Type {
	case TypeWorkspace:
		errs = append(errs, validateWorkspace(m)...)
	case TypeCollection:
		errs = append(errs, validateCollection(m)...)
	case TypeSkill:
		// no extra fields beyond skill section
	case TypeMCP:
		errs = append(errs, validateMCP(m)...)
	case TypeCLI:
		errs = append(errs, validateCLI(m)...)
	}

	errs = append(errs, validateKeywords(m)...)
	errs = append(errs, validateSource(m)...)
	errs = append(errs, validateInstall(m)...)
	errs = append(errs, validateUpstream(m)...)
	errs = append(errs, validateDependencies(m)...)

	return errs
}

// validateCore checks name, version, type, and description.
func validateCore(m *Manifest) []string {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if !nameRegex.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf("name %q must match @scope/name format (lowercase, alphanumeric, hyphens)", m.Name))
	}

	// Workspace type doesn't require version (it's not published itself).
	if m.Type == TypeWorkspace {
		if m.Version != "" && !semverRegex.MatchString(m.Version) {
			errs = append(errs, fmt.Sprintf("version %q must be valid semver (e.g. 1.0.0)", m.Version))
		}
	} else if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverRegex.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf("version %q must be valid semver (e.g. 1.0.0)", m.Version))
	}

	if m.Type == "" {
		errs = append(errs, "type is required")
	} else if !m.Type.Valid() {
		errs = append(errs, fmt.Sprintf("type %q must be one of: skill, mcp, cli, workspace, collection", m.Type))
	}

	if m.Description == "" {
		errs = append(errs, "description is required")
	} else if len(m.Description) > 1024 {
		errs = append(errs, "description must be 1024 characters or less")
	}

	return errs
}

// validateKeywords checks keyword count and length.
// Note: "env:" and "bin:" prefixed keywords are allowed — they originate from
// OpenClaw requires mappings and have no dedicated field yet.
func validateKeywords(m *Manifest) []string {
	var errs []string
	if len(m.Keywords) > 20 {
		errs = append(errs, fmt.Sprintf("keywords: %d items exceeds maximum of 20", len(m.Keywords)))
	}
	for i, kw := range m.Keywords {
		if len(kw) > 50 {
			errs = append(errs, fmt.Sprintf("keywords[%d]: %q exceeds 50 character limit", i, kw))
		}
	}
	return errs
}

// validateSkillSection checks skill entry and origin constraints.
func validateSkillSection(m *Manifest) []string {
	var errs []string

	// Skill section: required for skill/cli types, optional for MCP.
	if m.Type != TypeMCP && m.Type != TypeWorkspace && m.Type != TypeCollection {
		if m.Skill == nil || m.Skill.Entry == "" {
			errs = append(errs, "skill section with entry is required (agents need instructions)")
		}
	}
	if m.Skill != nil && m.Skill.Entry != "" {
		if filepath.IsAbs(m.Skill.Entry) || strings.Contains(filepath.Clean(m.Skill.Entry), "..") {
			errs = append(errs, "skill.entry must be a relative path within the project (no absolute paths or ..)")
		}
	}

	// Validate skill.origin
	if m.Skill != nil && m.Skill.Origin != "" {
		if m.Skill.Origin != "native" && m.Skill.Origin != "wrapped" {
			errs = append(errs, fmt.Sprintf("skill.origin %q must be 'native' or 'wrapped'", m.Skill.Origin))
		}
	}

	return errs
}

var validTransport = map[string]bool{"stdio": true, "sse": true, "http": true, "streamable-http": true}

// commandSafe performs basic safety checks on a command string.
func commandSafe(cmd string) bool {
	// Reject shell metacharacters and pipes that could enable injection.
	for _, c := range cmd {
		switch c {
		case '|', ';', '&', '$', '`', '(', ')', '{', '}', '<', '>':
			return false
		}
	}
	return true
}

// validateMCP checks the mcp section including transports and hooks.
func validateMCP(m *Manifest) []string {
	var errs []string

	if m.MCP == nil {
		return append(errs, "mcp section is required for type=mcp")
	}

	if m.MCP.Transport == "" {
		errs = append(errs, "mcp.transport is required (stdio, sse, or http)")
	} else if !validTransport[m.MCP.Transport] {
		errs = append(errs, fmt.Sprintf("mcp.transport %q must be one of: stdio, sse, http, streamable-http", m.MCP.Transport))
	}
	if m.MCP.Transport == "stdio" && m.MCP.Command == "" {
		errs = append(errs, "mcp.command is required for stdio transport")
	}
	if m.MCP.Command != "" && !commandSafe(m.MCP.Command) {
		errs = append(errs, fmt.Sprintf("mcp.command %q contains disallowed shell metacharacters", m.MCP.Command))
	}
	if (m.MCP.Transport == "sse" || m.MCP.Transport == "http" || m.MCP.Transport == "streamable-http") && m.MCP.URL == "" {
		errs = append(errs, "mcp.url is required for sse/http transport")
	}

	// Validate transports[]
	seenIDs := make(map[string]bool)
	for i, t := range m.MCP.Transports {
		if t.ID == "" {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].id is required", i))
		} else if seenIDs[t.ID] {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].id %q is duplicate", i, t.ID))
		} else {
			seenIDs[t.ID] = true
		}
		if t.Transport == "" {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].transport is required", i))
		} else if !validTransport[t.Transport] {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].transport %q must be one of: stdio, sse, http, streamable-http", i, t.Transport))
		}
		if t.Transport == "stdio" && t.Command == "" {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].command is required for stdio transport", i))
		}
		if t.Command != "" && !commandSafe(t.Command) {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].command %q contains disallowed shell metacharacters", i, t.Command))
		}
		if (t.Transport == "sse" || t.Transport == "http" || t.Transport == "streamable-http") && t.URL == "" {
			errs = append(errs, fmt.Sprintf("mcp.transports[%d].url is required for sse/http transport", i))
		}
	}

	// Validate hooks
	if m.MCP.Hooks != nil {
		for i, h := range m.MCP.Hooks.PostInstall {
			if h.Command == "" {
				errs = append(errs, fmt.Sprintf("mcp.hooks.post_install[%d].command is required", i))
			} else if !commandSafe(h.Command) {
				errs = append(errs, fmt.Sprintf("mcp.hooks.post_install[%d].command %q contains disallowed shell metacharacters", i, h.Command))
			}
		}
	}

	return errs
}

// ValidateMCPWarnings returns non-fatal warnings for MCP packages.
func ValidateMCPWarnings(m *Manifest) []string {
	var warnings []string
	if m.MCP == nil {
		return warnings
	}

	// Warn if stdio command like npx/node/python has no args — it won't run anything useful
	if m.MCP.Transport == "stdio" && m.MCP.Command != "" && len(m.MCP.Args) == 0 {
		launchers := map[string]bool{"npx": true, "node": true, "python": true, "python3": true, "deno": true, "bun": true}
		if launchers[m.MCP.Command] {
			warnings = append(warnings, fmt.Sprintf("mcp.args is empty but command is %q — add args (e.g. [\"-y\", \"package-name\"]) so the MCP server can start", m.MCP.Command))
		}
	}

	return warnings
}

// validateCLI checks the cli section.
func validateCLI(m *Manifest) []string {
	var errs []string

	if m.CLI == nil {
		return append(errs, "cli section is required for type=cli")
	}
	if m.CLI.Binary == "" {
		errs = append(errs, "cli.binary is required")
	}

	// Validate cli.compatible constraint
	if m.CLI.Compatible != "" {
		c := m.CLI.Compatible
		if !strings.ContainsAny(c, ">=<^~*") && !semverRegex.MatchString(c) {
			errs = append(errs, fmt.Sprintf("cli.compatible %q must be a valid semver constraint", c))
		}
	}

	// CLI packages must declare how to install the binary
	if m.Install == nil {
		errs = append(errs, "install section is required for type=cli (users need a way to obtain the binary)")
	} else if !hasInstallMethod(m.Install) {
		errs = append(errs, "install section must specify at least one install method (script, brew, npm, pip, gem, cargo, or platforms)")
	}

	return errs
}

// hasInstallMethod returns true if at least one concrete install method is declared.
func hasInstallMethod(i *InstallSpec) bool {
	if i.Source == "artifact" {
		return true
	}
	if i.Script != "" || i.Brew != "" || i.Npm != "" || i.Pip != "" || i.Gem != "" || i.Cargo != "" {
		return true
	}
	if len(i.Platforms) > 0 {
		return true
	}
	return false
}

// validateWorkspace checks the workspace section.
func validateWorkspace(m *Manifest) []string {
	var errs []string

	if m.Workspace == nil {
		return append(errs, "workspace section is required for type=workspace")
	}
	if len(m.Workspace.Members) == 0 {
		errs = append(errs, "workspace.members must have at least one glob pattern")
	}
	for _, c := range m.Workspace.Collections {
		if c.Name == "" {
			errs = append(errs, "workspace.collections[].name is required")
		}
		if len(c.Members) == 0 {
			errs = append(errs, fmt.Sprintf("workspace.collections[%q].members must not be empty", c.Name))
		}
	}
	if m.Skill != nil || m.MCP != nil || m.CLI != nil {
		errs = append(errs, "workspace type cannot have skill, mcp, or cli sections")
	}

	return errs
}

// validateCollection checks the collection section.
func validateCollection(m *Manifest) []string {
	var errs []string

	if m.Collection == nil || len(m.Collection.Members) == 0 {
		return append(errs, "collection.members is required for type=collection")
	}
	for _, member := range m.Collection.Members {
		if !nameRegex.MatchString(member) {
			errs = append(errs, fmt.Sprintf("collection member %q must be @scope/name format", member))
		}
	}
	if m.Skill != nil || m.MCP != nil || m.CLI != nil {
		errs = append(errs, "collection type cannot have skill, mcp, or cli sections")
	}

	return errs
}

// validateSource checks the source spec.
func validateSource(m *Manifest) []string {
	var errs []string

	if m.Source != nil {
		if m.Source.GitHub != "" && !githubRepoRegex.MatchString(m.Source.GitHub) {
			errs = append(errs, fmt.Sprintf("source.github %q must be owner/repo format", m.Source.GitHub))
		}
	}

	return errs
}

// validateInstall checks the install spec.
func validateInstall(m *Manifest) []string {
	var errs []string

	if m.Install != nil {
		if m.Install.Source != "" {
			if m.Install.Source != "artifact" &&
				!strings.HasPrefix(m.Install.Source, "github:") &&
				!strings.HasPrefix(m.Install.Source, "npm:") &&
				!strings.HasPrefix(m.Install.Source, "pip:") &&
				!strings.HasPrefix(m.Install.Source, "https://") {
				errs = append(errs, fmt.Sprintf("install.source %q must be \"artifact\" or start with github:, npm:, pip:, or https://", m.Install.Source))
			}
		}
		if m.Install.Script != "" && !strings.HasPrefix(m.Install.Script, "https://") {
			errs = append(errs, fmt.Sprintf("install.script %q must be an https:// URL", m.Install.Script))
		}
	}

	return errs
}

// validateUpstream checks the upstream spec.
func validateUpstream(m *Manifest) []string {
	var errs []string

	if m.Upstream != nil {
		if m.Upstream.NPM == "" && m.Upstream.GitHub == "" && m.Upstream.Docker == "" {
			errs = append(errs, "upstream must specify at least one of: npm, github, docker")
		}
		if m.Upstream.Tracking != "" {
			validTracking := map[string]bool{"npm": true, "github-release": true, "docker": true}
			if !validTracking[m.Upstream.Tracking] {
				errs = append(errs, fmt.Sprintf("upstream.tracking %q must be one of: npm, github-release, docker", m.Upstream.Tracking))
			}
		}
		if m.Upstream.VersionPattern != "" {
			// version_pattern must be "*" or a valid semver constraint prefix
			vp := m.Upstream.VersionPattern
			if vp != "*" && !strings.ContainsAny(vp, ">=<^~") && !semverRegex.MatchString(vp) {
				errs = append(errs, fmt.Sprintf("upstream.version_pattern %q must be \"*\" or a valid semver constraint", vp))
			}
		}
	}

	return errs
}

// validateDependencies checks dependency names and constraints.
func validateDependencies(m *Manifest) []string {
	var errs []string

	for dep, constraint := range m.Dependencies {
		if !nameRegex.MatchString(dep) {
			errs = append(errs, fmt.Sprintf("dependency %q has invalid name", dep))
		}
		if constraint == "" {
			errs = append(errs, fmt.Sprintf("dependency %q must have a version constraint", dep))
		}
	}

	return errs
}

// Scaffold generates a minimal ctx.yaml template for a given type.
func Scaffold(pkgType PackageType, scope, name string) *Manifest {
	m := &Manifest{
		Name:        FormatFullName(scope, name),
		Version:     "0.1.0",
		Type:        pkgType,
		Description: fmt.Sprintf("A %s package", pkgType),
	}

	switch pkgType {
	case TypeSkill:
		m.Skill = &SkillSpec{Entry: "SKILL.md"}
	case TypeMCP:
		// MCP servers are self-describing — skill is optional
		m.MCP = &MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Args:      []string{"dist/index.js"},
		}
	case TypeCLI:
		m.Skill = &SkillSpec{Entry: fmt.Sprintf("skills/%s/SKILL.md", name)}
		m.CLI = &CLISpec{
			Binary: name,
			Verify: fmt.Sprintf("%s --version", name),
		}
		m.Install = &InstallSpec{
			Script: fmt.Sprintf("https://raw.githubusercontent.com/OWNER/%s/main/scripts/install.sh", name),
		}
	}

	return m
}

// Marshal serializes a manifest to YAML bytes.
func Marshal(m *Manifest) ([]byte, error) {
	return yaml.Marshal(m)
}
