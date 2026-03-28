package main

import (
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <package@version>",
	Short: "Switch to a locally installed version",
	Long: `Switch the active version of a package to another locally installed version.
This is instant — no download needed. The version must already be installed locally.

Examples:
  ctx use @hong/review@1.0.0     Switch to version 1.0.0
  ctx use @hong/review@1.1.0     Switch back to 1.1.0`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		ref := args[0]

		// Parse @scope/name@version
		fullName, version, err := parsePackageRef(ref)
		if err != nil {
			return output.ErrUsageHint(err.Error(), "Example: ctx use @scope/name@1.0.0")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		// Check version exists locally
		versions := inst.InstalledVersions(fullName)
		found := false
		for _, v := range versions {
			if v == version {
				found = true
				break
			}
		}
		if !found {
			return output.ErrUsageHint(
				"version "+version+" is not installed locally",
				"Run 'ctx i "+fullName+"@"+version+"' to install it first",
			)
		}

		// Get current version before switch
		oldVersion := inst.CurrentVersion(fullName)

		// Switch
		pkgDir := inst.PackageDir(fullName)
		if err := installer.SwitchCurrent(pkgDir, version); err != nil {
			return err
		}

		result := map[string]string{
			"full_name":   fullName,
			"old_version": oldVersion,
			"new_version": version,
		}

		summary := fullName + ": " + oldVersion + " → " + version
		if oldVersion == "" {
			summary = fullName + ": → " + version
		}

		return w.OK(result,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list versions", Command: "ctx ls " + fullName + " --versions", Description: "List installed versions"},
				output.Breadcrumb{Action: "prune", Command: "ctx prune " + fullName, Description: "Clean old versions"},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
}
