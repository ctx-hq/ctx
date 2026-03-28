package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/getctx/ctx/internal/resolver"
	"github.com/spf13/cobra"
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
  ctx install github:user/repo         Install from GitHub directly`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		output.Info("Resolving %s...", args[0])
		result, err := inst.Install(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		// Run type-specific post-install actions
		if err := runPostInstall(cmd, result); err != nil {
			output.Warn("Post-install: %v", err)
		}

		action := "installed"
		if !result.IsNew {
			action = "updated"
		}

		return w.OK(result,
			output.WithSummary(result.FullName+"@"+result.Version+" "+action),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info " + result.FullName, Description: "View package details"},
				output.Breadcrumb{Action: "list", Command: "ctx ls", Description: "List installed packages"},
			),
		)
	},
}

// runPostInstall performs type-specific actions after a package is installed.
func runPostInstall(cmd *cobra.Command, result *installer.InstallResult) error {
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

	switch manifest.PackageType(result.Type) {
	case manifest.TypeCLI:
		return installer.InstallCLI(cmd.Context(), &m)
	case manifest.TypeSkill:
		return installer.LinkSkillToAgents(result.InstallPath, m.ShortName(), result.FullName)
	case manifest.TypeMCP:
		return installer.LinkMCPToAgents(&m)
	}
	return nil
}
