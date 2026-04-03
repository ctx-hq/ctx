package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/profile"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

// getToken returns the current auth token.
// Priority: CTX_TOKEN env var > profile-resolved keychain token > empty string.
// The env var enables CI/CD workflows (e.g. GitHub Actions) where
// interactive login is not possible.
func getToken() string {
	if envToken := os.Getenv("CTX_TOKEN"); envToken != "" {
		return envToken
	}

	res, err := profile.Resolve(flagProfile)
	if err != nil {
		if !errors.Is(err, profile.ErrNoProfile) {
			// A specific profile was referenced (flag/env/.ctx-profile) but not found — warn.
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		return ""
	}

	token, err := auth.GetProfileToken(res.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return ""
	}
	return token
}

// resolvedProfile returns the resolved profile result.
// Returns nil if no profile is configured (not an error for commands that don't require auth).
func resolvedProfile() *profile.ResolveResult {
	res, err := profile.Resolve(flagProfile)
	if err != nil {
		return nil
	}
	return res
}

// resolvedUsername returns the username from the resolved profile, or "".
func resolvedUsername() string {
	res := resolvedProfile()
	if res == nil {
		return ""
	}
	return res.Profile.Username
}

// resolvedRegistryURL returns the registry URL from the resolved profile,
// falling back to the config's registry, then the default.
func resolvedRegistryURL() string {
	res := resolvedProfile()
	if res != nil && res.Profile.Registry != "" {
		return res.Profile.Registry
	}
	// Fall back to config registry or env override
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultRegistry
	}
	return cfg.RegistryURL()
}

// authedRegistry returns a Writer and authenticated registry Client.
// It checks online status, auth token, and loads config in one call.
func authedRegistry(cmd *cobra.Command) (*output.Writer, *registry.Client, error) {
	if err := requireOnline(); err != nil {
		return nil, nil, err
	}
	w := getWriter(cmd)
	token := getToken()
	if token == "" {
		return nil, nil, output.ErrAuth("not logged in")
	}
	registryURL := resolvedRegistryURL()
	return w, registry.New(registryURL, token), nil
}

// requireOnline returns an error if offline mode is active (via --offline flag
// or network_mode=offline in config). Call this at the top of commands that
// require network access.
func requireOnline() error {
	if flagOffline {
		return fmt.Errorf("this command requires network access (--offline is set)")
	}
	cfg, err := config.Load()
	if err != nil {
		return nil // can't load config → assume online
	}
	if cfg.IsOffline() {
		return fmt.Errorf("this command requires network access (network_mode=offline)")
	}
	return nil
}

// requireProfile resolves the current profile and returns an error if not logged in.
func requireProfile() (*profile.ResolveResult, error) {
	res, err := profile.Resolve(flagProfile)
	if err != nil {
		if errors.Is(err, profile.ErrNoProfile) {
			return nil, output.ErrAuth("not logged in")
		}
		return nil, err
	}
	return res, nil
}
