package main

import (
	"fmt"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var (
	searchType     string
	searchPlatform string
	searchLimit    int
)

var searchCmd = &cobra.Command{
	Use:     "search <query>",
	Aliases: []string{"s"},
	Short:   "Search for packages",
	Long: `Search skills, MCP servers, and CLI tools.

Examples:
  ctx search "code review"
  ctx search "git" --type mcp
  ctx search "file search" --type cli --platform darwin`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		result, err := reg.Search(cmd.Context(), args[0], searchType, searchPlatform, searchLimit)
		if err != nil {
			return err
		}

		return w.OK(result.Packages,
			output.WithSummary(fmt.Sprintf("%d results", result.Total)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "info", Command: "ctx info <package>", Description: "View package details"},
				output.Breadcrumb{Action: "install", Command: "ctx i <package>", Description: "Install a package"},
			),
		)
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchType, "type", "t", "", "Filter by type (skill, mcp, cli)")
	searchCmd.Flags().StringVarP(&searchPlatform, "platform", "p", "", "Filter by platform")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 20, "Max results")
}
