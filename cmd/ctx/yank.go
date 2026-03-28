package main

import (
	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var yankCmd = &cobra.Command{
	Use:   "yank <package@version>",
	Short: "Yank (retract) a published version",
	Long: `Mark a published version as yanked. Yanked versions are hidden from
resolution but remain downloadable for existing installations.

Examples:
  ctx yank @hong/my-skill@1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		ref := args[0]

		// Parse @scope/name@version
		fullName, version, err := parsePackageRef(ref)
		if err != nil {
			return output.ErrUsageHint(err.Error(), "Example: ctx yank @scope/name@1.0.0")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsLoggedIn() {
			return output.ErrAuth("not logged in")
		}

		if !flagYes {
			output.Warn("This will yank %s@%s. Existing installations are not affected.", fullName, version)
			return output.ErrUsageHint(
				"confirmation required",
				"Run with --yes to confirm",
			)
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		if err := reg.Yank(cmd.Context(), fullName, version); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"yanked": "true", "full_name": fullName, "version": version},
			output.WithSummary("Yanked "+fullName+"@"+version),
		)
	},
}

func init() {
	rootCmd.AddCommand(yankCmd)
}
