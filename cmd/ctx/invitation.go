package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var invitationsCmd = &cobra.Command{
	Use:   "invitations",
	Short: "Manage your organization invitations",
	Long: `View and respond to organization invitations.

Examples:
  ctx invitations                 List your pending invitations
  ctx invitations accept <id>     Accept an invitation
  ctx invitations decline <id>    Decline an invitation`,
	Aliases: []string{"inv"},
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
		invitations, err := reg.ListMyInvitations(cmd.Context())
		if err != nil {
			return err
		}

		return w.OK(invitations,
			output.WithSummary(fmt.Sprintf("%d pending invitations", len(invitations))),
			output.WithBreadcrumbs(
				breadcrumbsForInvitations(invitations)...,
			),
		)
	},
}

func breadcrumbsForInvitations(invitations []registry.OrgInvitation) []output.Breadcrumb {
	if len(invitations) == 0 {
		return nil
	}
	inv := invitations[0]
	return []output.Breadcrumb{
		{Action: "accept", Command: "ctx invitations accept " + inv.ID, Description: "Accept invitation to @" + inv.OrgName},
		{Action: "decline", Command: "ctx invitations decline " + inv.ID, Description: "Decline invitation"},
	}
}

var invitationsAcceptCmd = &cobra.Command{
	Use:   "accept <invitation-id>",
	Short: "Accept an organization invitation",
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
		if err := reg.AcceptInvitation(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"accepted": args[0]},
			output.WithSummary("Accepted invitation "+args[0]),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "view orgs", Command: "ctx org list", Description: "List your organizations"},
			),
		)
	},
}

var invitationsDeclineCmd = &cobra.Command{
	Use:   "decline <invitation-id>",
	Short: "Decline an organization invitation",
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
		if err := reg.DeclineInvitation(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"declined": args[0]},
			output.WithSummary("Declined invitation "+args[0]),
		)
	},
}

func init() {
	invitationsCmd.AddCommand(invitationsAcceptCmd)
	invitationsCmd.AddCommand(invitationsDeclineCmd)
	rootCmd.AddCommand(invitationsCmd)
}
