package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"gopkg.in/yaml.v3"
)

// SaveToken stores the auth token in the system keychain (or file fallback)
// and saves the username in config.
func SaveToken(token, username string) error {
	kc := getKeychain()
	if err := kc.Set(keychainService, keychainAccount, token); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Username = username
	return cfg.Save()
}

// ClearToken removes the auth token from keychain and clears username from config.
func ClearToken() error {
	kc := getKeychain()
	if err := kc.Delete(keychainService, keychainAccount); err != nil {
		return fmt.Errorf("failed to remove token from keychain: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Username = ""
	return cfg.Save()
}

// GetToken returns the current auth token from the system keychain.
// Returns ("", nil) if no token is stored; returns a non-nil error
// if the keychain is inaccessible (e.g. locked, permission denied).
func GetToken() (string, error) {
	kc := getKeychain()
	token, err := kc.Get(keychainService, keychainAccount)
	if err != nil {
		// "not found" style errors mean no token stored — not an error.
		if isNotFoundErr(err) {
			// Try one-time migration from legacy config.yaml token field.
			if migrated := migrateFromConfig(); migrated != "" {
				return migrated, nil
			}
			return "", nil
		}
		return "", fmt.Errorf("keychain access: %w", err)
	}
	return token, nil
}

// migrateFromConfig performs a one-time migration of the auth token from the
// legacy config.yaml `token` field to the system keychain. Returns the migrated
// token, or "" if there was nothing to migrate.
func migrateFromConfig() string {
	path := config.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Parse into a loose map to access the removed "token" field.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	tok, ok := raw["token"].(string)
	if !ok || tok == "" {
		return ""
	}

	// Migrate: store in keychain, then remove the field from config.yaml.
	kc := getKeychain()
	if err := kc.Set(keychainService, keychainAccount, tok); err != nil {
		// Can't write to keychain — return the token anyway so the user
		// isn't silently logged out, but don't remove it from config.
		return tok
	}

	// Remove the token field and rewrite the config file.
	delete(raw, "token")
	out, err := yaml.Marshal(raw)
	if err == nil {
		_ = os.WriteFile(path, out, config.FilePerm)
	}
	return tok
}

// isNotFoundErr returns true if the error indicates "credential not found"
// rather than a keychain access failure.
func isNotFoundErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no credentials") ||
		strings.Contains(msg, "could not be found") ||
		strings.Contains(msg, "SecKeychainSearchCopyNext") // macOS "no matching item"
}
