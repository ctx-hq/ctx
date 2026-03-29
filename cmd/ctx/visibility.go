package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var visibilityCmd = &cobra.Command{
	Use:   "visibility <package> [public|unlisted|private]",
	Short: "View or change package visibility",
	Long: `View or change the visibility of a package.

Examples:
  ctx visibility @scope/name              View current visibility
  ctx visibility @scope/name public       Make package public
  ctx visibility @scope/name private      Make package private`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)

		if len(args) == 1 {
			// View current visibility
			pkg, err := reg.GetPackage(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			visibility := pkg.Visibility
			if visibility == "" {
				visibility = "unknown"
			}
			return w.OK(
				map[string]string{"package": args[0], "visibility": visibility},
				output.WithSummary(fmt.Sprintf("%s visibility: %s", args[0], visibility)),
			)
		}

		// Set visibility
		visibility := args[1]
		if visibility != "public" && visibility != "unlisted" && visibility != "private" {
			return output.ErrUsage("visibility must be public, unlisted, or private")
		}

		if token == "" {
			return output.ErrAuth("not logged in")
		}

		// TODO: PATCH /v1/packages/:fullName with visibility field
		return fmt.Errorf("visibility change is not yet implemented (coming soon)")
	},
}
