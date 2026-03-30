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

		roleStr, _ := cmd.Flags().GetString("role")
		role := strings.ToLower(roleStr)
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

var orgInviteCmd = &cobra.Command{
	Use:   "invite <org> <username>",
	Short: "Invite a user to join an organization",
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

		roleStr, _ := cmd.Flags().GetString("role")
		role := strings.ToLower(roleStr)
		if role != "owner" && role != "admin" && role != "member" {
			return output.ErrUsage("role must be owner, admin, or member")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		inv, err := reg.InviteOrgMember(cmd.Context(), args[0], args[1], role)
		if err != nil {
			return err
		}

		return w.OK(inv,
			output.WithSummary(fmt.Sprintf("Invited %s to @%s as %s", args[1], args[0], role)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "check", Command: "ctx org invitations " + args[0], Description: "View pending invitations"},
			),
		)
	},
}

var orgInvitationsCmd = &cobra.Command{
	Use:   "invitations <org>",
	Short: "List invitations for an organization",
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
		invitations, err := reg.ListOrgInvitations(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(invitations,
			output.WithSummary(fmt.Sprintf("%d invitations for @%s", len(invitations), args[0])),
		)
	},
}

var orgCancelInviteCmd = &cobra.Command{
	Use:   "cancel-invite <org> <invitation-id>",
	Short: "Cancel a pending invitation",
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
		if err := reg.CancelOrgInvitation(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"org": args[0], "cancelled": args[1]},
			output.WithSummary(fmt.Sprintf("Cancelled invitation %s in @%s", args[1], args[0])),
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
			return output.ErrUsageHint(
				fmt.Sprintf("this will permanently delete organization @%s", args[0]),
				"Run with --yes to confirm",
			)
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

var orgArchiveCmd = &cobra.Command{
	Use:   "archive <name>",
	Short: "Archive an organization (freeze publishing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.ArchiveOrg(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"archived": args[0]},
			output.WithSummary("Archived organization @"+args[0]),
		)
	},
}

var orgUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <name>",
	Short: "Unarchive an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.UnarchiveOrg(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"unarchived": args[0]},
			output.WithSummary("Unarchived organization @"+args[0]),
		)
	},
}

var orgLeaveCmd = &cobra.Command{
	Use:   "leave <name>",
	Short: "Leave an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.LeaveOrg(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"left": args[0]},
			output.WithSummary("Left organization @"+args[0]),
		)
	},
}

var orgRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if !flagYes {
			return output.ErrUsageHint(
				fmt.Sprintf("this will rename organization @%s to @%s", args[0], args[1]),
				"Run with --yes to confirm",
			)
		}

		result, err := reg.RenameOrg(cmd.Context(), args[0], args[1], args[0])
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Renamed organization @%s → @%s", args[0], args[1])),
		)
	},
}

var orgDissolveAction string
var orgDissolveTransferTo string

var orgDissolveCmd = &cobra.Command{
	Use:   "dissolve <name>",
	Short: "Dissolve an organization",
	Long: `Dissolve an organization. Requires --yes to confirm.

Use --action to specify what happens to packages:
  delete      Delete all packages (default)
  transfer    Transfer packages to another scope (requires --transfer-to)

Examples:
  ctx org dissolve myteam --yes
  ctx org dissolve myteam --action transfer --transfer-to @alice --yes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		switch orgDissolveAction {
		case "delete":
			// ok
		case "transfer":
			if orgDissolveTransferTo == "" {
				return output.ErrUsageHint(
					"--transfer-to is required when using --action transfer",
					"Example: ctx org dissolve myteam --action transfer --transfer-to @alice --yes",
				)
			}
		default:
			return output.ErrUsageHint(
				fmt.Sprintf("unknown action %q", orgDissolveAction),
				"Valid actions: delete, transfer",
			)
		}

		if !flagYes {
			return output.ErrUsageHint(
				fmt.Sprintf("this will permanently dissolve organization @%s", args[0]),
				"Run with --yes to confirm",
			)
		}

		if err := reg.DissolveOrg(cmd.Context(), args[0], orgDissolveAction, orgDissolveTransferTo, args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"dissolved": args[0]},
			output.WithSummary("Dissolved organization @"+args[0]),
		)
	},
}

func init() {
	orgCreateCmd.Flags().StringVar(&orgDisplayName, "display-name", "", "Display name for the organization")
	orgAddCmd.Flags().String("role", "member", "Member role (owner, admin, member)")

	orgInviteCmd.Flags().String("role", "member", "Member role (owner, admin, member)")

	orgDissolveCmd.Flags().StringVar(&orgDissolveAction, "action", "delete", "Action for packages (delete, transfer)")
	orgDissolveCmd.Flags().StringVar(&orgDissolveTransferTo, "transfer-to", "", "Target scope for package transfer (with --action transfer)")

	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgInfoCmd)
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgPackagesCmd)
	orgCmd.AddCommand(orgAddCmd)
	orgCmd.AddCommand(orgRemoveCmd)
	orgCmd.AddCommand(orgDeleteCmd)
	orgCmd.AddCommand(orgInviteCmd)
	orgCmd.AddCommand(orgInvitationsCmd)
	orgCmd.AddCommand(orgCancelInviteCmd)
	orgCmd.AddCommand(orgArchiveCmd)
	orgCmd.AddCommand(orgUnarchiveCmd)
	orgCmd.AddCommand(orgLeaveCmd)
	orgCmd.AddCommand(orgRenameCmd)
	orgCmd.AddCommand(orgDissolveCmd)
	rootCmd.AddCommand(orgCmd)
}
