package main

import (
	"context"
	"fmt"
	"time"

	"github.com/getctx/ctx/internal/auth"
	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/output"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with getctx.org via GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.IsLoggedIn() {
			output.Info("Already logged in as %s", cfg.Username)
			output.PrintDim("  Run 'ctx logout' to sign out")
			return nil
		}

		output.Info("Starting GitHub authentication...")

		resp, err := auth.StartDeviceFlow(cmd.Context(), cfg.RegistryURL())
		if err != nil {
			return fmt.Errorf("start auth: %w", err)
		}

		fmt.Println()
		fmt.Printf("  Open:  %s\n", resp.VerificationURI)
		fmt.Printf("  Code:  %s%s%s\n", output.Bold, resp.UserCode, output.Reset)
		fmt.Println()
		output.Info("Waiting for authorization...")

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(resp.ExpiresIn)*time.Second)
		defer cancel()

		token, err := auth.PollForToken(ctx, cfg.RegistryURL(), resp.DeviceCode, resp.Interval)
		if err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}

		if err := auth.SaveToken(token.AccessToken, ""); err != nil {
			return fmt.Errorf("save token: %w", err)
		}

		output.Success("Authenticated successfully!")
		return nil
	},
}
