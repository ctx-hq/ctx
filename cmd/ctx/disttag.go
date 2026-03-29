package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var distTagCmd = &cobra.Command{
	Use:     "dist-tag",
	Aliases: []string{"tag"},
	Short:   "Manage distribution tags",
	Long: `Manage dist-tags (named pointers to versions) for packages.

Examples:
  ctx dist-tag ls @scope/name
  ctx dist-tag add @scope/name beta 2.0.0-beta.1
  ctx dist-tag rm @scope/name beta`,
}

var distTagLsCmd = &cobra.Command{
	Use:   "ls <package>",
	Short: "List dist-tags",
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
		tags, err := reg.ListTags(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		return w.OK(tags, output.WithSummary(fmt.Sprintf("%d tags for %s", len(tags), args[0])))
	},
}

var distTagAddCmd = &cobra.Command{
	Use:   "add <package> <tag> <version>",
	Short: "Set a dist-tag to point to a version",
	Args:  cobra.ExactArgs(3),
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
		if err := reg.SetTag(cmd.Context(), args[0], args[1], args[2]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"package": args[0], "tag": args[1], "version": args[2]},
			output.WithSummary(fmt.Sprintf("Set %s → %s on %s", args[1], args[2], args[0])),
		)
	},
}

var distTagRmCmd = &cobra.Command{
	Use:   "rm <package> <tag>",
	Short: "Remove a dist-tag",
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
		if err := reg.DeleteTag(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}

		return w.OK(
			map[string]string{"package": args[0], "tag": args[1]},
			output.WithSummary(fmt.Sprintf("Removed tag %s from %s", args[1], args[0])),
		)
	},
}

func init() {
	distTagCmd.AddCommand(distTagLsCmd)
	distTagCmd.AddCommand(distTagAddCmd)
	distTagCmd.AddCommand(distTagRmCmd)
}
