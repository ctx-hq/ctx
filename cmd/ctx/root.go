package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"

	// Global flags
	flagJSON    bool
	flagQuiet   bool
	flagStyled  bool
	flagMD      bool
	flagIDsOnly bool
	flagCount   bool
	flagAgent   bool
	flagYes     bool
	flagOffline bool
)

// writerKey is the context key for the output Writer.
type writerKeyType struct{}

var writerKey = writerKeyType{}

// getWriter retrieves the Writer from the command context.
func getWriter(cmd *cobra.Command) *output.Writer {
	if w, ok := cmd.Context().Value(writerKey).(*output.Writer); ok {
		return w
	}
	// Fallback: create a default writer
	return output.NewWriter()
}

var rootCmd = &cobra.Command{
	Use:   "ctx",
	Short: "The universal context package manager for LLM agents",
	Long: `ctx manages skills, MCP servers, and CLI tools for AI agents.

It's a bridge layer — ctx knows where to find packages and how to install them,
delegating to native package managers (brew, npm, pip, cargo) when appropriate.

  ctx install @scope/name     Install a package
  ctx search "query"          Search for packages
  ctx publish                 Publish a package
  ctx serve                   Run as MCP server (for agent use)`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Set version for User-Agent in HTTP requests
		config.Version = Version

		// Resolve output format from flags
		format, err := output.ResolveFormat(flagJSON, flagQuiet, flagStyled, flagMD, flagIDsOnly, flagCount, flagAgent)
		if err != nil {
			return err
		}

		// Create Writer and attach to context
		w := output.NewWriter(output.WithFormat(format))
		ctx := context.WithValue(cmd.Context(), writerKey, w)
		cmd.SetContext(ctx)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Show update notice after every command (at most once per 24h).
		// Skip for machine-readable output, CI environments, or the upgrade command itself.
		cfg, _ := config.Load()
		offline := flagOffline || (cfg != nil && cfg.IsOffline())
		updateEnabled := cfg == nil || cfg.IsUpdateCheckEnabled()
		if !flagAgent && !flagQuiet && !flagJSON && !offline && updateEnabled && os.Getenv("CI") == "" && cmd.Name() != "upgrade" {
			if latest := selfupdate.CheckForUpdate(Version); latest != "" {
				fmt.Fprintf(os.Stderr, "\n\033[0;33mnotice:\033[0m ctx %s available (current: %s)\n", latest, Version)
				fmt.Fprintf(os.Stderr, "  run \033[1mctx upgrade\033[0m to update\n")
			}
		}
		return nil
	},
}

func init() {
	// Output format flags (mutually exclusive)
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON envelope")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Output data only, no envelope")
	rootCmd.PersistentFlags().BoolVar(&flagStyled, "styled", false, "Force styled output (ANSI colors)")
	rootCmd.PersistentFlags().BoolVar(&flagMD, "md", false, "Output as Markdown")
	rootCmd.PersistentFlags().BoolVar(&flagIDsOnly, "ids-only", false, "Output one ID per line")
	rootCmd.PersistentFlags().BoolVar(&flagCount, "count", false, "Output count only")
	rootCmd.PersistentFlags().BoolVar(&flagAgent, "agent", false, "Agent mode (quiet output, JSON errors)")

	// Behavior flags
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&flagOffline, "offline", false, "Disable all network access")

	rootCmd.AddCommand(
		installCmd,
		removeCmd,
		searchCmd,
		infoCmd,
		listCmd,
		publishCmd,
		pushCmd,
		loginCmd,
		initCmd,
		validateCmd,
		versionCmd,
		upgradeCmd,
		distTagCmd,
		syncCmd,
		visibilityCmd,
		enrichCmd,
		whoamiCmd,
	)
}
