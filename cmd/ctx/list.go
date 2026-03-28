package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

var listType string

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		if listType != "" && listType != "skill" && listType != "mcp" && listType != "cli" {
			return output.ErrUsage("--type must be skill, mcp, or cli")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		entries, err := inst.ScanInstalled()
		if err != nil {
			return err
		}

		// Filter by type
		if listType != "" {
			filtered := make([]installer.InstalledPackage, 0)
			for _, e := range entries {
				if e.Type == listType {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		return w.OK(entries,
			output.WithSummary(fmt.Sprintf("%d packages installed", len(entries))),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info <package>", Description: "View package details"},
				output.Breadcrumb{Action: "update", Command: "ctx up", Description: "Update all packages"},
			),
		)
	},
}

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by type (skill, mcp, cli)")
}
