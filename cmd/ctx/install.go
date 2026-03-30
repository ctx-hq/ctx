package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

var flagCaller string

var installCmd = &cobra.Command{
	Use:     "install <package[@version]>",
	Aliases: []string{"i"},
	Short:   "Install a package",
	Long: `Install a skill, MCP server, or CLI tool.

Examples:
  ctx install @hong/my-skill           Install latest version
  ctx install @hong/my-skill@^1.0      Install with constraint
  ctx install @mcp/github@2.1.0        Install exact version
  ctx install github:user/repo         Install from GitHub directly`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		output.Info("Resolving %s...", args[0])
		result, err := inst.Install(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		// Run type-specific post-install actions (linking)
		if err := runPostInstall(cmd, result, flagCaller); err != nil {
			output.Warn("Post-install: %v", err)
		}

		// Show description if available
		description := loadDescription(result)
		if description != "" {
			output.PrintDim("\n  %s", description)
		}

		// Show reload hint based on package type
		hint := reloadHint(result.Type)
		// CLI packages with bundled skills also need a reload hint
		if hint == "" && manifest.PackageType(result.Type) == manifest.TypeCLI && hasSkillMD(result.InstallPath) {
			hint = "Start a new conversation to use the bundled skill"
		}
		if hint != "" {
			output.Info(hint)
		}

		action := "installed"
		if !result.IsNew {
			action = "updated"
		}

		opts := []output.ResponseOption{
			output.WithSummary(result.FullName + "@" + result.Version + " " + action),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info " + result.FullName, Description: "View package details"},
				output.Breadcrumb{Action: "list", Command: "ctx ls", Description: "List installed packages"},
			),
		}
		if hint != "" {
			opts = append(opts, output.WithNotice(hint))
		}

		return w.OK(result, opts...)
	},
}

func init() {
	installCmd.Flags().StringVar(&flagCaller, "caller", "", "Agent that invoked this install (e.g., claude, cursor)")
}

// runPostInstall performs type-specific actions after a package is installed.
func runPostInstall(cmd *cobra.Command, result *installer.InstallResult, caller string) error {
	// Load the manifest from the installed package
	manifestPath := filepath.Join(result.InstallPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil // no manifest, nothing to do
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	// Collect state for tracking
	pkgDir := filepath.Dir(result.InstallPath)
	state := &installstate.PackageState{
		FullName:    result.FullName,
		Version:     result.Version,
		Type:        result.Type,
		InstalledAt: time.Now().UTC(),
	}

	switch manifest.PackageType(result.Type) {
	case manifest.TypeCLI:
		// Script installs require explicit user confirmation
		if m.Install != nil && m.Install.Script != "" && !flagYes {
			output.Warn("This package installs via shell script: %s", m.Install.Script)
			p := prompt.DefaultPrompter()
			ok, err := p.Confirm("Execute this script?", false)
			if err != nil || !ok {
				output.Warn("script installation cancelled by user")
			} else {
				cliState, err := installer.InstallCLI(cmd.Context(), &m)
				if err != nil {
					output.Warn("CLI install: %v", err)
				} else {
					state.CLI = cliState
				}
			}
		} else if m.Install != nil && m.Install.Script != "" {
			cliState, err := installer.InstallCLI(cmd.Context(), &m)
			if err != nil {
				output.Warn("CLI install: %v", err)
			} else {
				state.CLI = cliState
			}
		}

		// CLI packages may bundle a SKILL.md — link it to agents
		if hasSkillMD(result.InstallPath) {
			skillStates, linkErr := installer.LinkSkillToAgents(result.InstallPath, m.ShortName(), result.FullName, caller)
			if linkErr != nil {
				output.Warn("Skill linking: %v", linkErr)
			}
			state.Skills = skillStates
		}

		// Show auth hint
		if m.CLI != nil && m.CLI.Auth != "" {
			output.Warn(m.CLI.Auth)
		}

	case manifest.TypeSkill:
		skillStates, linkErr := installer.LinkSkillToAgents(result.InstallPath, m.ShortName(), result.FullName, caller)
		if linkErr != nil {
			output.Warn("Skill linking: %v", linkErr)
		}
		state.Skills = skillStates

	case manifest.TypeMCP:
		mcpStates, err := installer.LinkMCPToAgents(&m)
		if err != nil {
			output.Warn("MCP config: %v", err)
		}
		state.MCP = mcpStates
		// MCP packages may bundle a SKILL.md — link it to agents
		if hasSkillMD(result.InstallPath) {
			skillStates, linkErr := installer.LinkSkillToAgents(result.InstallPath, m.ShortName(), result.FullName, caller)
			if linkErr != nil {
				output.Warn("Skill linking: %v", linkErr)
			}
			state.Skills = skillStates
		}
	}

	// Save installation state for repair/uninstall
	_ = state.Save(pkgDir) // best effort

	return nil
}

// hasSkillMD checks if a package directory contains a SKILL.md file.
func hasSkillMD(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
}

// loadDescription reads the package description from the installed manifest.
func loadDescription(result *installer.InstallResult) string {
	manifestPath := filepath.Join(result.InstallPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.Description
}

// reloadHint returns a user-facing hint about reloading based on package type.
func reloadHint(pkgType string) string {
	switch manifest.PackageType(pkgType) {
	case manifest.TypeSkill:
		return "Start a new conversation to use this skill"
	case manifest.TypeMCP:
		return "Restart your agent to load this MCP server"
	default:
		return ""
	}
}
