package initdetect

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// ToManifest converts a DetectResult into a ctx Manifest.
func ToManifest(r *DetectResult) *manifest.Manifest {
	m := &manifest.Manifest{
		Name:        inferCtxName(r),
		Version:     r.Version,
		Type:        r.PackageType,
		Description: r.Description,
		License:     r.License,
		Homepage:    r.Homepage,
		Repository:  r.Repository,
		Keywords:    r.Keywords,
		Upstream:    r.Upstream,
	}

	switch r.PackageType {
	case manifest.TypeMCP:
		if r.MCP != nil {
			m.MCP = &manifest.MCPSpec{
				Transport:  r.MCP.Transport,
				Command:    r.MCP.Command,
				Args:       r.MCP.Args,
				URL:        r.MCP.URL,
				Env:        r.MCP.Env,
				Tools:      r.MCP.Tools,
				Resources:  r.MCP.Resources,
				Require:    r.MCP.Require,
				Hooks:      r.MCP.Hooks,
				Transports: r.MCP.Transports,
			}
		}
	case manifest.TypeCLI:
		if r.CLI != nil {
			m.Skill = &manifest.SkillSpec{
				Entry:  fmt.Sprintf("skills/%s/SKILL.md", r.Name),
				Origin: "wrapped",
			}
			m.CLI = &manifest.CLISpec{
				Binary: r.CLI.Binary,
				Verify: r.CLI.Verify,
			}
			m.Install = r.CLI.Install
		}
	case manifest.TypeSkill:
		m.Skill = &manifest.SkillSpec{
			Entry: "SKILL.md",
		}
	}

	return m
}

// inferCtxName generates a ctx-compatible @scope/name from the detection result.
func inferCtxName(r *DetectResult) string {
	// Determine scope based on type
	var scope string
	switch r.PackageType {
	case manifest.TypeMCP:
		scope = "mcp"
	case manifest.TypeCLI:
		scope = "cli"
	default:
		scope = "community"
	}

	// Clean up name
	name := r.Name
	origScope := ""
	// Remove existing scope if present (e.g., @playwright/mcp -> use "playwright" as name)
	if strings.HasPrefix(name, "@") {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 {
			origScope = strings.TrimPrefix(parts[0], "@")
			shortName := parts[1]
			// If the short name is generic (like "mcp"), use the scope as the name
			if shortName == "mcp" || shortName == "cli" || shortName == "server" {
				name = origScope
			} else {
				name = shortName
			}
		}
	}
	// Remove common suffixes/prefixes for cleaner names
	name = strings.TrimSuffix(name, "-mcp-server")
	name = strings.TrimSuffix(name, "-mcp")
	name = strings.TrimPrefix(name, "mcp-server-")
	name = strings.TrimPrefix(name, "mcp-")

	if name == "" {
		name = "unknown"
	}

	return fmt.Sprintf("@%s/%s", scope, name)
}
