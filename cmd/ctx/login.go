package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/profile"
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

		// Determine target profile name via resolve chain when --profile is not set.
		// This respects CTX_PROFILE, .ctx-profile, and the active profile.
		profileName := flagProfile
		if profileName == "" {
			if res, err := profile.Resolve(""); err == nil {
				profileName = res.Name
			} else {
				profileName = "default"
			}
		}
		if err := profile.ValidateName(profileName); err != nil {
			return err
		}

		// Resolve the registry URL: prefer the existing profile's registry,
		// then fall back to global config, then the default.
		registryURL := config.DefaultRegistry
		store, _ := profile.Load()
		if store != nil {
			if p, ok := store.Profiles[profileName]; ok && p.Registry != "" {
				registryURL = p.Registry
			}
		}
		if registryURL == config.DefaultRegistry {
			if cfg, err := config.Load(); err == nil {
				registryURL = cfg.RegistryURL()
			}
		}

		// Check if already logged in to this profile
		existingToken, _ := auth.GetProfileToken(profileName)
		if existingToken != "" {
			username := ""
			if store != nil {
				if p, ok := store.Profiles[profileName]; ok {
					username = p.Username
				}
			}
			if username != "" {
				output.Info("Already logged in as %s (profile: %s)", username, profileName)
			} else {
				output.Info("Already logged in (profile: %s)", profileName)
			}
			output.PrintDim("  Run 'ctx logout' to sign out")
			return nil
		}

		output.Info("Starting GitHub authentication...")
		resp, err := auth.StartDeviceFlow(cmd.Context(), registryURL)
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

		token, err := auth.PollForToken(ctx, registryURL, resp.DeviceCode, resp.Interval)
		if err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}

		// Save token to profile-scoped keychain key
		if err := auth.SaveProfileToken(profileName, token.AccessToken); err != nil {
			return fmt.Errorf("save token: %w", err)
		}

		// Best-effort: fetch username with a short timeout
		username := ""
		meCtx, meCancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer meCancel()
		client := registry.New(registryURL, token.AccessToken)
		if me, err := client.GetMe(meCtx); err == nil {
			username = me.Username
		}

		// Create/update profile entry — preserve existing registry if the profile
		// already has one, so re-login doesn't silently overwrite a custom registry.
		store, loadErr := profile.Load()
		if loadErr != nil {
			store = &profile.ProfileStore{
				Profiles: make(map[string]*profile.Profile),
			}
		}

		if existing, ok := store.Profiles[profileName]; ok {
			existing.Username = username
			// Keep existing.Registry unchanged — it was already used for this login.
		} else {
			store.Profiles[profileName] = &profile.Profile{
				Username: username,
				Registry: registryURL,
			}
		}

		// If this is the first profile or no active profile, set it active
		if store.Active == "" || len(store.Profiles) == 1 {
			store.Active = profileName
		}

		if err := store.Save(); err != nil {
			return fmt.Errorf("save profile: %w", err)
		}

		if username != "" {
			output.Success("Logged in as %s (profile: %s)", username, profileName)
		} else {
			output.Success("Authenticated successfully! (profile: %s)", profileName)
		}
		return nil
	},
}
