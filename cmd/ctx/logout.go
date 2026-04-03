package main

import (
	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/profile"
	"github.com/spf13/cobra"
)

// LogoutInfo is the response data for the logout command.
type LogoutInfo struct {
	Username string `json:"username,omitempty"`
	Profile  string `json:"profile"`
	Registry string `json:"registry"`
	Status   string `json:"status"` // "logged_out" or "already_logged_out"
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of getctx.org",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		store, err := profile.Load()
		if err != nil {
			return err
		}

		// Determine which profile to log out from
		profileName := flagProfile
		if profileName == "" {
			// Use current active profile
			res, resolveErr := profile.Resolve(flagProfile)
			if resolveErr != nil {
				return w.OK(
					&LogoutInfo{
						Status: "already_logged_out",
					},
					output.WithSummary("Already logged out"),
					output.WithBreadcrumbs(
						output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
					),
				)
			}
			profileName = res.Name
		}

		p, ok := store.Profiles[profileName]
		if !ok {
			return w.OK(
				&LogoutInfo{
					Profile: profileName,
					Status:  "already_logged_out",
				},
				output.WithSummary("Already logged out"),
				output.WithBreadcrumbs(
					output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
				),
			)
		}

		previousUsername := p.Username
		registryURL := p.RegistryURL()

		// Clear auth token from keychain
		if err := auth.ClearProfileToken(profileName); err != nil {
			return err
		}

		// Remove profile from store
		delete(store.Profiles, profileName)
		if store.Active == profileName {
			store.Active = ""
		}
		if err := store.Save(); err != nil {
			return err
		}

		summary := "Logged out"
		if previousUsername != "" {
			summary = "Logged out (was: " + previousUsername + ", profile: " + profileName + ")"
		}

		return w.OK(
			&LogoutInfo{
				Username: previousUsername,
				Profile:  profileName,
				Registry: registryURL,
				Status:   "logged_out",
			},
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
			),
		)
	},
}
