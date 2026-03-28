package main

import (
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
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
