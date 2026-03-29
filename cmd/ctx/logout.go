package main

import (
	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

// LogoutInfo is the response data for the logout command.
type LogoutInfo struct {
	Username string `json:"username,omitempty"`
	Registry string `json:"registry"`
	Status   string `json:"status"` // "logged_out" or "already_logged_out"
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of getctx.org",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		previousUsername := cfg.Username
		registry := cfg.RegistryURL()

		// Use auth.GetToken() directly to distinguish "no token" from "keychain error".
		// The getToken() helper swallows errors and returns "", which would
		// cause us to report "already logged out" when the keychain is actually
		// inaccessible (and the token may still be stored).
		token, tokenErr := auth.GetToken()
		if tokenErr != nil {
			return tokenErr
		}
		if token == "" && previousUsername == "" {
			return w.OK(
				&LogoutInfo{
					Registry: registry,
					Status:   "already_logged_out",
				},
				output.WithSummary("Already logged out"),
				output.WithBreadcrumbs(
					output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
				),
			)
		}

		// Clear auth token from keychain and username from config
		if err := auth.ClearToken(); err != nil {
			return err
		}

		summary := "Logged out"
		if previousUsername != "" {
			summary = "Logged out (was: " + previousUsername + ")"
		}

		return w.OK(
			&LogoutInfo{
				Username: previousUsername,
				Registry: registry,
				Status:   "logged_out",
			},
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
			),
		)
	},
}
