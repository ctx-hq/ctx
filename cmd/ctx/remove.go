package main

import (
	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/getctx/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <package>",
	Aliases: []string{"rm", "uninstall"},
	Short:   "Remove an installed package",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		if err := inst.Remove(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(map[string]string{"removed": args[0]},
			output.WithSummary("Removed "+args[0]),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx ls", Description: "List installed packages"},
				output.Breadcrumb{Action: "install", Command: "ctx i " + args[0], Description: "Reinstall package"},
			),
		)
	},
}
