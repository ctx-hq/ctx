package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var starRemove bool

var starCmd = &cobra.Command{
	Use:   "star <package>",
	Short: "Star or unstar a package",
	Long: `Star a package to bookmark it for later.

Examples:
  ctx star @scope/package           Star a package
  ctx star @scope/package --remove  Unstar a package
  ctx star list                     List your starred packages
  ctx star list create "my-tools"   Create a star list`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		fullName := args[0]

		if starRemove {
			if err := reg.UnstarPackage(cmd.Context(), fullName); err != nil {
				return err
			}
			return w.OK(
				map[string]string{"unstarred": fullName},
				output.WithSummary(fmt.Sprintf("Unstarred %s", fullName)),
				output.WithBreadcrumbs(
					output.Breadcrumb{Action: "list", Command: "ctx star list", Description: "View starred packages"},
				),
			)
		}

		if err := reg.StarPackage(cmd.Context(), fullName); err != nil {
			return err
		}
		return w.OK(
			map[string]string{"starred": fullName},
			output.WithSummary(fmt.Sprintf("Starred %s", fullName)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx star list", Description: "View starred packages"},
				output.Breadcrumb{Action: "info", Command: "ctx info " + fullName, Description: "View package details"},
			),
		)
	},
}

var starListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List your starred packages",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		stars, err := reg.ListStars(cmd.Context())
		if err != nil {
			return err
		}

		return w.OK(stars,
			output.WithSummary(fmt.Sprintf("%d starred packages", len(stars))),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "star", Command: "ctx star <package>", Description: "Star a package"},
			),
		)
	},
}

var starListCreateVisibility string

var starListCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a star list",
	Long: `Create a curated list to organize your starred packages.

Examples:
  ctx star list create "my-ai-tools"
  ctx star list create "data-stack" --public`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		result, err := reg.CreateStarList(cmd.Context(), args[0], starListCreateVisibility)
		if err != nil {
			return err
		}

		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Created list %q (%s)", result.Name, result.Visibility)),
		)
	},
}

var starListShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show your star lists",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		lists, err := reg.ListStarLists(cmd.Context())
		if err != nil {
			return err
		}

		return w.OK(lists,
			output.WithSummary(fmt.Sprintf("%d star lists", len(lists))),
		)
	},
}

func init() {
	starCmd.Flags().BoolVar(&starRemove, "remove", false, "Unstar the package")

	starListCreateCmd.Flags().StringVar(&starListCreateVisibility, "public", "private", "List visibility: private or public")

	starListCmd.AddCommand(starListCreateCmd)
	starListCmd.AddCommand(starListShowCmd)
	starCmd.AddCommand(starListCmd)
	rootCmd.AddCommand(starCmd)
}
