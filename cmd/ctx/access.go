package main

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var accessCmd = &cobra.Command{
	Use:   "access <package>",
	Short: "Manage package access control",
	Long: `View and manage per-user access for private packages.

Examples:
  ctx access @scope/pkg                     List users with access
  ctx access grant @scope/pkg alice bob     Grant access to users
  ctx access revoke @scope/pkg alice        Revoke access from users`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		entries, err := reg.GetPackageAccess(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(entries,
			output.WithSummary(fmt.Sprintf("%d users with access to %s", len(entries), args[0])),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "grant", Command: "ctx access grant " + args[0] + " <user>", Description: "Grant access"},
				output.Breadcrumb{Action: "revoke", Command: "ctx access revoke " + args[0] + " <user>", Description: "Revoke access"},
			),
		)
	},
}

var accessGrantCmd = &cobra.Command{
	Use:   "grant <package> <user> [user...]",
	Short: "Grant users access to a private package",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		pkg := args[0]
		users := args[1:]
		reg := registry.New(cfg.RegistryURL(), token)
		if err := reg.UpdatePackageAccess(cmd.Context(), pkg, users, nil); err != nil {
			return err
		}

		return w.OK(
			map[string]any{"package": pkg, "granted": users},
			output.WithSummary(fmt.Sprintf("Granted access to %s: %s", pkg, strings.Join(users, ", "))),
		)
	},
}

var accessRevokeCmd = &cobra.Command{
	Use:   "revoke <package> <user> [user...]",
	Short: "Revoke users' access to a private package",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		pkg := args[0]
		users := args[1:]
		reg := registry.New(cfg.RegistryURL(), token)
		if err := reg.UpdatePackageAccess(cmd.Context(), pkg, nil, users); err != nil {
			return err
		}

		return w.OK(
			map[string]any{"package": pkg, "revoked": users},
			output.WithSummary(fmt.Sprintf("Revoked access from %s: %s", pkg, strings.Join(users, ", "))),
		)
	},
}

func init() {
	accessCmd.AddCommand(accessGrantCmd)
	accessCmd.AddCommand(accessRevokeCmd)
	rootCmd.AddCommand(accessCmd)
}
