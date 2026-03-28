package main

import (
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <package>",
	Short: "Show package details",
	Args:  cobra.ExactArgs(1),
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
		pkg, err := reg.GetPackage(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(pkg,
			output.WithSummary(pkg.FullName+"@"+pkg.Version),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "install", Command: "ctx i " + pkg.FullName, Description: "Install this package"},
				output.Breadcrumb{Action: "remove", Command: "ctx rm " + pkg.FullName, Description: "Remove this package"},
			),
		)
	},
}
