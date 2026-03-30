package main

import (
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

// getToken returns the current auth token from keychain or config fallback.
// Returns empty string if not logged in. Prints a warning to stderr on
// keychain access errors so the user is aware of the issue.
func getToken() string {
	token, err := auth.GetToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return ""
	}
	return token
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
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	return w, registry.New(cfg.RegistryURL(), token), nil
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
