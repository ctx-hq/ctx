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

	// Skill section: required for skill/cli types, optional for MCP.
	// MCP servers are self-describing via tools/list, so SKILL.md is optional
	// (but recommended for providing additional context to agents).
	// Workspace and collection types don't have skill sections.
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

	// Type-specific validation
	switch m.Type {
	case TypeWorkspace:
		if m.Workspace == nil {
			errs = append(errs, "workspace section is required for type=workspace")
		} else {
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
		}
		if m.Skill != nil || m.MCP != nil || m.CLI != nil {
			errs = append(errs, "workspace type cannot have skill, mcp, or cli sections")
		}
	case TypeCollection:
		if m.Collection == nil || len(m.Collection.Members) == 0 {
			errs = append(errs, "collection.members is required for type=collection")
		} else {
			for _, member := range m.Collection.Members {
				if !nameRegex.MatchString(member) {
					errs = append(errs, fmt.Sprintf("collection member %q must be @scope/name format", member))
				}
			}
		}
		if m.Skill != nil || m.MCP != nil || m.CLI != nil {
			errs = append(errs, "collection type cannot have skill, mcp, or cli sections")
		}
	case TypeSkill:
		// no extra fields beyond skill section
	case TypeMCP:
		if m.MCP == nil {
			errs = append(errs, "mcp section is required for type=mcp")
		} else {
			if m.MCP.Transport == "" {
				errs = append(errs, "mcp.transport is required (stdio, sse, or http)")
			} else {
				validTransport := map[string]bool{"stdio": true, "sse": true, "http": true, "streamable-http": true}
				if !validTransport[m.MCP.Transport] {
					errs = append(errs, fmt.Sprintf("mcp.transport %q must be one of: stdio, sse, http, streamable-http", m.MCP.Transport))
				}
			}
			if m.MCP.Transport == "stdio" && m.MCP.Command == "" {
				errs = append(errs, "mcp.command is required for stdio transport")
			}
			if (m.MCP.Transport == "sse" || m.MCP.Transport == "http" || m.MCP.Transport == "streamable-http") && m.MCP.URL == "" {
				errs = append(errs, "mcp.url is required for sse/http transport")
			}
		}
	case TypeCLI:
		if m.CLI == nil {
			errs = append(errs, "cli section is required for type=cli")
		} else if m.CLI.Binary == "" {
			errs = append(errs, "cli.binary is required")
		}
	}

	// Validate skill.origin (applicable to any type that carries a skill section)
	if m.Skill != nil && m.Skill.Origin != "" {
		if m.Skill.Origin != "native" && m.Skill.Origin != "wrapped" {
			errs = append(errs, fmt.Sprintf("skill.origin %q must be 'native' or 'wrapped'", m.Skill.Origin))
		}
	}

	// Validate source spec (adapter packages)
	if m.Source != nil {
		if m.Source.GitHub != "" && !githubRepoRegex.MatchString(m.Source.GitHub) {
			errs = append(errs, fmt.Sprintf("source.github %q must be owner/repo format", m.Source.GitHub))
		}
	}

	// Validate cli.compatible constraint (if present)
	if m.CLI != nil && m.CLI.Compatible != "" {
		// Basic check: should look like a semver constraint
		c := m.CLI.Compatible
		if !strings.ContainsAny(c, ">=<^~*") && !semverRegex.MatchString(c) {
			errs = append(errs, fmt.Sprintf("cli.compatible %q must be a valid semver constraint", c))
		}
	}

	// Validate install spec
	if m.Install != nil {
		if m.Install.Source != "" {
			if !strings.HasPrefix(m.Install.Source, "github:") &&
				!strings.HasPrefix(m.Install.Source, "npm:") &&
				!strings.HasPrefix(m.Install.Source, "pip:") &&
				!strings.HasPrefix(m.Install.Source, "https://") {
				errs = append(errs, fmt.Sprintf("install.source %q must start with github:, npm:, pip:, or https://", m.Install.Source))
			}
		}
		if m.Install.Script != "" && !strings.HasPrefix(m.Install.Script, "https://") {
			errs = append(errs, fmt.Sprintf("install.script %q must be an https:// URL", m.Install.Script))
		}
	}

	// Validate dependency constraints
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
		m.Install = &InstallSpec{}
	}

	return m
}

// Marshal serializes a manifest to YAML bytes.
func Marshal(m *Manifest) ([]byte, error) {
	return yaml.Marshal(m)
}
