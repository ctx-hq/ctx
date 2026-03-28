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

var outdatedCmd = &cobra.Command{
	Use:     "outdated",
	Aliases: []string{"od"},
	Short:   "Check for available updates",
	Long: `Compare installed package versions against the latest available.

Shows packages that have newer versions available in the registry.`,
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

		entries, err := inst.ScanInstalled()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return w.OK([]any{}, output.WithSummary("no packages installed"))
		}

		type OutdatedEntry struct {
			FullName string `json:"full_name"`
			Current  string `json:"current"`
			Latest   string `json:"latest"`
			Type     string `json:"type"`
		}

		var outdated []OutdatedEntry
		for _, e := range entries {
			pkg, err := reg.GetPackage(cmd.Context(), e.FullName)
			if err != nil {
				continue // skip if registry unreachable
			}
			if pkg.Version != "" && pkg.Version != e.Version {
				outdated = append(outdated, OutdatedEntry{
					FullName: e.FullName,
					Current:  e.Version,
					Latest:   pkg.Version,
					Type:     e.Type,
				})
			}
		}

		summary := "all packages are up to date"
		if len(outdated) > 0 {
			summary = fmt.Sprintf("%d update(s) available", len(outdated))
		}

		return w.OK(outdated,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "update", Command: "ctx up", Description: "Update all packages"},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(outdatedCmd)
}
