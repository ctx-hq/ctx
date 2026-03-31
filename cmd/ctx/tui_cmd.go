package main

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	tuiapp "github.com/ctx-hq/ctx/internal/tui/app"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive package browser",
	Long:  "Launch an interactive terminal UI for browsing, searching, and managing packages.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check TTY
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return output.ErrUsage("TUI requires an interactive terminal")
		}

		// Check format compatibility
		w := getWriter(cmd)
		if w.IsMachine() {
			return output.ErrUsage("TUI is incompatible with --json, --quiet, or --agent flags")
		}

		// Load config
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Get auth token
		token, _ := auth.GetToken()

		// Create registry client and installer
		reg := registry.New(cfg.RegistryURL(), token)
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		// Create service
		svc := &tuiapp.RealService{
			Installer: inst,
			Registry:  reg,
			Version:   Version,
			Token:     token,
		}

		// Create and run program
		m := tuiapp.New(svc)
		p := tea.NewProgram(m)
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
