package main

import (
	"os"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Publish a package to the registry",
	Long: `Publish a package defined by ctx.yaml to getctx.org.

Reads ctx.yaml from the current directory (or specified path),
validates it, and uploads to the registry.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		// Load and validate manifest
		m, err := manifest.LoadFromDir(dir)
		if err != nil {
			return err
		}

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			return output.ErrUsageHint(
				"validation failed: "+errs[0],
				"Fix errors and try again",
			)
		}

		// Check auth
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsLoggedIn() {
			return output.ErrAuth("not logged in")
		}

		// Marshal manifest
		data, err := manifest.Marshal(m)
		if err != nil {
			return err
		}

		// Publish
		reg := registry.New(cfg.RegistryURL(), cfg.Token)

		output.Info("Publishing %s@%s...", m.Name, m.Version)

		// Open archive file if it exists
		var archive *os.File
		archivePath := dir + "/package.tar.gz"
		if f, err := os.Open(archivePath); err == nil {
			archive = f
			defer archive.Close()
		}

		result, err := reg.Publish(cmd.Context(), data, archive)
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary("Published "+result.FullName+"@"+result.Version),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info " + result.FullName, Description: "View package"},
			),
		)
	},
}
