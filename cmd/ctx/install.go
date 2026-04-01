package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/secrets"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/ctx-hq/ctx/internal/tui/inline"
	"github.com/spf13/cobra"
)

var (
	flagCaller string
	flagPick   bool
)

var installCmd = &cobra.Command{
	Use:     "install <package[@version]>",
	Aliases: []string{"i"},
	Short:   "Install a package",
	Long: `Install a skill, MCP server, or CLI tool.

Examples:
  ctx install @hong/my-skill           Install latest version
  ctx install @hong/my-skill@^1.0      Install with constraint
  ctx install @mcp/github@2.1.0        Install exact version
  ctx install github:user/repo         Install from GitHub directly
  ctx install @baoyu/skills            Install a collection (all members)
  ctx install @baoyu/skills --pick     Install selected members from collection`,
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

		// Install with progress bar when TTY is available
		var result *installer.InstallResult
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		if isTTY && !flagYes && !w.IsMachine() {
			err = inline.RunWithProgress(cmd.Context(), "Installing "+args[0], func(_ context.Context, report func(float64)) error {
				report(0.1)
				r, installErr := inst.Install(cmd.Context(), args[0])
				if installErr != nil {
					return installErr
				}
				result = r
				report(1.0)
				return nil
			})
		} else {
			output.Info("Resolving %s...", args[0])
			result, err = inst.Install(cmd.Context(), args[0])
		}
		if err != nil {
			return err
		}

		// Collection expansion: if the installed package is a collection,
		// install all member packages.
		if collectionManifest := loadCollectionManifest(result); collectionManifest != nil {
			return installCollection(cmd, result, collectionManifest, inst, w)
		}

		// Resolve caller: --caller flag takes precedence, fallback to CTX_CALLER env
		caller := flagCaller
		if caller == "" {
			caller = os.Getenv("CTX_CALLER")
		}

		// Agent selection: let user pick target agents when TTY and not --yes
		var selectedAgents []agent.Agent
		if isTTY && !flagYes && !w.IsMachine() && caller == "" {
			agents := agent.DetectAll()
			if len(agents) > 1 {
				selected, selectErr := inline.SelectAgents(agents)
				if selectErr == nil && selected != nil {
					selectedAgents = selected
				}
			}
		}

		// Run type-specific post-install actions (linking)
		if err := runPostInstall(cmd, result, caller, selectedAgents); err != nil {
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
			output.WithMeta("type", result.Type),
			output.WithMeta("source", result.Source),
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
	installCmd.Flags().BoolVar(&flagPick, "pick", false, "Interactively select members from a collection")
}

// runPostInstall performs type-specific actions after a package is installed.
func runPostInstall(cmd *cobra.Command, result *installer.InstallResult, caller string, targetAgents []agent.Agent) error {
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
			skillStates, linkErr := installer.LinkSkillToAgents(cmd.Context(), result.InstallPath, m.ShortName(), result.FullName, caller, targetAgents)
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
		skillStates, linkErr := installer.LinkSkillToAgents(cmd.Context(), result.InstallPath, m.ShortName(), result.FullName, caller, targetAgents)
		if linkErr != nil {
			output.Warn("Skill linking: %v", linkErr)
		}
		state.Skills = skillStates

	case manifest.TypeMCP:
		// Prompt for required env vars that are not yet stored
		var missingEnvVars []string // collected inside the block, read after it
		if m.MCP != nil && len(m.MCP.Env) > 0 {
			store, loadErr := secrets.Load()
			if loadErr != nil {
				store = secrets.New()
			}
			isTTYForEnv := term.IsTerminal(int(os.Stdin.Fd()))
			changed := false
			for _, e := range m.MCP.Env {
				if !e.Required {
					continue
				}
				if _, ok := store.Get(m.Name, e.Name); ok {
					continue
				}
				if isTTYForEnv && !flagYes {
					p := prompt.DefaultPrompter()
					label := e.Name
					if e.Description != "" {
						label += " (" + e.Description + ")"
					}
					val, promptErr := p.Text(label, e.Default)
					if promptErr == nil && val != "" {
						store.Set(m.Name, e.Name, val)
						changed = true
						continue
					}
				}
				// Still missing after prompt attempt (skipped, empty, or non-TTY)
				missingEnvVars = append(missingEnvVars, e.Name)
			}
			if changed {
				_ = store.Save()
			}
		}

		mcpStates, err := installer.LinkMCPToAgents(cmd.Context(), &m)
		if err != nil {
			output.Warn("MCP config: %v", err)
		}
		state.MCP = mcpStates

		// MCP packages may bundle a SKILL.md — link silently (MCP config already printed agent list)
		if hasSkillMD(result.InstallPath) {
			quietCtx := context.WithValue(cmd.Context(), installer.QuietLinkKey, true)
			skillStates, linkErr := installer.LinkSkillToAgents(quietCtx, result.InstallPath, m.ShortName(), result.FullName, caller, targetAgents)
			if linkErr != nil {
				output.Warn("Skill linking: %v", linkErr)
			}
			state.Skills = skillStates
		}

		// Post-install guidance
		output.PrintDim("")
		if len(missingEnvVars) > 0 {
			output.Warn("Required environment variables not set:")
			for _, name := range missingEnvVars {
				output.PrintDim("    %s", name)
			}
			output.Info("Set them with:")
			for _, name := range missingEnvVars {
				output.PrintDim("    ctx mcp env set %s %s=<value>", m.Name, name)
			}
			output.Info("Then re-link to agents:")
			output.PrintDim("    ctx install %s", m.Name)
			output.PrintDim("")
		}
		output.PrintDim("  Next: ctx mcp test %s", m.ShortName())
	}

	// Save installation state for repair/uninstall
	if err := state.Save(pkgDir); err != nil {
		output.Warn("Failed to save install state: %v", err)
	}

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

// loadCollectionManifest checks if an install result is a collection package
// and returns its manifest if so. Returns nil for non-collection packages.
func loadCollectionManifest(result *installer.InstallResult) *manifest.Manifest {
	manifestPath := filepath.Join(result.InstallPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if m.Type != manifest.TypeCollection || m.Collection == nil || len(m.Collection.Members) == 0 {
		return nil
	}
	return &m
}

// installCollection installs all members of a collection package.
func installCollection(cmd *cobra.Command, result *installer.InstallResult, m *manifest.Manifest, inst *installer.Installer, w *output.Writer) error {
	members := m.Collection.Members

	// Interactive pick mode: let user select which members to install.
	if flagPick {
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		if isTTY {
			p := prompt.DefaultPrompter()
			var selected []string
			for _, member := range members {
				ok, pErr := p.Confirm("Install "+member+"?", true)
				if pErr != nil {
					return pErr
				}
				if ok {
					selected = append(selected, member)
				}
			}
			if len(selected) == 0 {
				output.Info("No members selected.")
				return nil
			}
			members = selected
		}
	}

	output.Info("Installing %d member(s) from collection %s...", len(members), result.FullName)

	caller := flagCaller
	if caller == "" {
		caller = os.Getenv("CTX_CALLER")
	}

	// Agent selection: let user pick target agents once for all members.
	var selectedAgents []agent.Agent
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	if isTTY && !flagYes && !w.IsMachine() && caller == "" {
		agents := agent.DetectAll()
		if len(agents) > 1 {
			selected, selectErr := inline.SelectAgents(agents)
			if selectErr == nil && selected != nil {
				selectedAgents = selected
			}
		}
	}

	var installed int
	var failed int
	for i, memberName := range members {
		output.Info("[%d/%d] Installing %s...", i+1, len(members), memberName)

		memberResult, installErr := inst.Install(cmd.Context(), memberName)
		if installErr != nil {
			output.Warn("Failed to install %s: %v", memberName, installErr)
			failed++
			continue
		}

		// Run post-install for each member.
		if postErr := runPostInstall(cmd, memberResult, caller, selectedAgents); postErr != nil {
			output.Warn("Post-install %s: %v", memberName, postErr)
		}

		installed++
	}

	summary := fmt.Sprintf("Installed %d/%d members from %s", installed, len(members), result.FullName)
	if failed > 0 {
		summary += fmt.Sprintf(" (%d failed)", failed)
	}

	return w.OK(map[string]interface{}{
		"collection": result.FullName,
		"installed":  installed,
		"failed":     failed,
		"total":      len(members),
	}, output.WithSummary(summary))
}
