package main

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
	Long: `Create and manage organizations for team-based package publishing.

Examples:
  ctx org create myteam
  ctx org info myteam
  ctx org add myteam alice --role admin
  ctx org remove myteam alice`,
}

var orgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return output.ErrAuth("not logged in")
		}

		// TODO: POST /v1/orgs with body {name: args[0]}
		return fmt.Errorf("org create is not yet implemented")
	},
}

var orgInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show organization details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load()
		if err != nil {
			return err
		}

		// TODO: GET /v1/orgs/:name
		return fmt.Errorf("org info is not yet implemented")
	},
}

var orgAddMemberRole string

var orgAddCmd = &cobra.Command{
	Use:   "add <org> <username>",
	Short: "Add a member to an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return output.ErrAuth("not logged in")
		}

		role := strings.ToLower(orgAddMemberRole)
		if role != "owner" && role != "admin" && role != "member" {
			return fmt.Errorf("role must be owner, admin, or member")
		}

		// TODO: POST /v1/orgs/:name/members with body {username, role}
		return fmt.Errorf("org add is not yet implemented")
	},
}

var orgRemoveCmd = &cobra.Command{
	Use:   "remove <org> <username>",
	Short: "Remove a member from an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return output.ErrAuth("not logged in")
		}

		// TODO: DELETE /v1/orgs/:name/members/:username
		return fmt.Errorf("org remove is not yet implemented")
	},
}

func init() {
	orgAddCmd.Flags().StringVar(&orgAddMemberRole, "role", "member", "Member role (owner, admin, member)")

	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgInfoCmd)
	orgCmd.AddCommand(orgAddCmd)
	orgCmd.AddCommand(orgRemoveCmd)
	rootCmd.AddCommand(orgCmd)
}
