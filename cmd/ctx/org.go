package main

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
	Long: `Create and manage organizations for team-based package publishing.

Examples:
  ctx org create myteam
  ctx org info myteam
  ctx org list
  ctx org packages myteam
  ctx org add myteam alice --role admin
  ctx org remove myteam alice
  ctx org delete myteam`,
}

var orgDisplayName string

var orgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new organization",
	Args:  cobra.ExactArgs(1),
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
		result, err := reg.CreateOrg(cmd.Context(), args[0], orgDisplayName)
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary("Created organization @"+args[0]),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish to @" + args[0]},
				output.Breadcrumb{Action: "add member", Command: "ctx org add " + args[0] + " <user>", Description: "Add team members"},
			),
		)
	},
}

var orgInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show organization details",
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
		result, err := reg.GetOrg(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary(fmt.Sprintf("@%s — %d members, %d packages", args[0], result.Members, result.Packages)),
		)
	},
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your organizations",
	Aliases: []string{"ls"},
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
		orgs, err := reg.ListMyOrgs(cmd.Context())
		if err != nil {
			return err
		}

		return w.OK(orgs, output.WithSummary(fmt.Sprintf("%d organizations", len(orgs))))
	},
}

var orgPackagesCmd = &cobra.Command{
	Use:   "packages <name>",
	Short: "List packages in an organization",
	Aliases: []string{"pkgs"},
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
		pkgs, err := reg.ListOrgPackages(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(pkgs, output.WithSummary(fmt.Sprintf("%d packages in @%s", len(pkgs), args[0])))
	},
}

var orgAddMemberRole string

var orgAddCmd = &cobra.Command{
	Use:   "add <org> <username>",
	Short: "Add a member to an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		role := strings.ToLower(orgAddMemberRole)
		if role != "owner" && role != "admin" && role != "member" {
			return output.ErrUsage("role must be owner, admin, or member")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		if err := reg.AddOrgMember(cmd.Context(), args[0], args[1], role); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"org": args[0], "username": args[1], "role": role},
			output.WithSummary(fmt.Sprintf("Added %s to @%s as %s", args[1], args[0], role)),
		)
	},
}

var orgRemoveCmd = &cobra.Command{
	Use:   "remove <org> <username>",
	Short: "Remove a member from an organization",
	Args:  cobra.ExactArgs(2),
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
		if err := reg.RemoveOrgMember(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"org": args[0], "removed": args[1]},
			output.WithSummary(fmt.Sprintf("Removed %s from @%s", args[1], args[0])),
		)
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an organization (must have 0 packages)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		if !flagYes {
			output.Warn("This will permanently delete organization @%s", args[0])
			output.Info("Use --yes to confirm")
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		if err := reg.DeleteOrg(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"deleted": args[0]},
			output.WithSummary("Deleted organization @"+args[0]),
		)
	},
}

func init() {
	orgCreateCmd.Flags().StringVar(&orgDisplayName, "display-name", "", "Display name for the organization")
	orgAddCmd.Flags().StringVar(&orgAddMemberRole, "role", "member", "Member role (owner, admin, member)")

	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgInfoCmd)
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgPackagesCmd)
	orgCmd.AddCommand(orgAddCmd)
	orgCmd.AddCommand(orgRemoveCmd)
	orgCmd.AddCommand(orgDeleteCmd)
	rootCmd.AddCommand(orgCmd)
}
