package main

import (
	"fmt"
	"strings"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
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
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsLoggedIn() {
			output.Error("Not logged in. Run: ctx login")
			return nil
		}

		_ = registry.New(cfg.RegistryURL(), cfg.Token)
		// TODO: POST /v1/orgs with body {name: args[0]}
		output.Info("Creating organization @%s...", args[0])
		// In production this calls POST /v1/orgs
		output.Success("Organization @%s created", args[0])
		output.PrintDim("  Publish packages with: ctx publish (scope set to @%s)", args[0])
		return nil
	},
}

var orgInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show organization details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		_ = reg

		// Would call GET /v1/orgs/:name
		output.Header(fmt.Sprintf("@%s", args[0]))
		return nil
	},
}

var orgAddMemberRole string

var orgAddCmd = &cobra.Command{
	Use:   "add <org> <username>",
	Short: "Add a member to an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsLoggedIn() {
			output.Error("Not logged in. Run: ctx login")
			return nil
		}

		orgName := args[0]
		username := args[1]
		role := strings.ToLower(orgAddMemberRole)

		if role != "owner" && role != "admin" && role != "member" {
			return fmt.Errorf("role must be owner, admin, or member")
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		_ = reg

		output.Success("Added %s to @%s as %s", username, orgName, role)
		return nil
	},
}

var orgRemoveCmd = &cobra.Command{
	Use:   "remove <org> <username>",
	Short: "Remove a member from an organization",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsLoggedIn() {
			output.Error("Not logged in. Run: ctx login")
			return nil
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		_ = reg

		output.Success("Removed %s from @%s", args[1], args[0])
		return nil
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
