package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with getctx.org via GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if getToken() != "" {
			output.Info("Already logged in as %s", cfg.Username)
			output.PrintDim("  Run 'ctx logout' to sign out")
			return nil
		}

		output.Info("Starting GitHub authentication...")

		resp, err := auth.StartDeviceFlow(cmd.Context(), cfg.RegistryURL())
		if err != nil {
			return fmt.Errorf("start auth: %w", err)
		}

		// Auto-open browser (best-effort — show URL as fallback)
		browserURL := resp.BrowserURL()
		if err := auth.OpenBrowser(browserURL); err != nil {
			output.PrintDim("  Could not open browser automatically")
		}

		fmt.Println()
		fmt.Printf("  Open:  %s\n", resp.VerificationURI)
		fmt.Printf("  Code:  %s%s%s\n", output.Bold, resp.UserCode, output.Reset)
		if resp.VerificationURIComplete != "" {
			fmt.Printf("\n  Or visit: %s\n", resp.VerificationURIComplete)
		}
		fmt.Println()
		output.Info("Waiting for authorization...")

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(resp.ExpiresIn)*time.Second)
		defer cancel()

		token, err := auth.PollForToken(ctx, cfg.RegistryURL(), resp.DeviceCode, resp.Interval)
		if err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}

		// Save token immediately so auth is persisted even if GetMe fails
		if err := auth.SaveToken(token.AccessToken, ""); err != nil {
			return fmt.Errorf("save token: %w", err)
		}

		// Best-effort: fetch username with a short timeout
		username := ""
		meCtx, meCancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer meCancel()
		client := registry.New(cfg.RegistryURL(), token.AccessToken)
		if me, err := client.GetMe(meCtx); err == nil {
			username = me.Username
			// Update saved token with username
			_ = auth.SaveToken(token.AccessToken, username)
		}

		if username != "" {
			output.Success("Logged in as %s", username)
		} else {
			output.Success("Authenticated successfully!")
		}
		return nil
	},
}
