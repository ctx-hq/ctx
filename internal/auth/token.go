package auth

import (
	"fmt"
	"strings"
)

// profileAccount returns the keychain account name for a named profile.
func profileAccount(name string) string {
	return "profile:" + name
}

// SaveProfileToken stores the auth token for a named profile in the keychain.
func SaveProfileToken(profileName, token string) error {
	kc := getKeychain()
	return kc.Set(keychainService, profileAccount(profileName), token)
}

// GetProfileToken returns the auth token for a named profile from the keychain.
// Returns ("", nil) if no token is stored.
func GetProfileToken(profileName string) (string, error) {
	kc := getKeychain()
	token, err := kc.Get(keychainService, profileAccount(profileName))
	if err != nil {
		if isNotFoundErr(err) {
			return "", nil
		}
		return "", fmt.Errorf("keychain access: %w", err)
	}
	return token, nil
}

// ClearProfileToken removes the auth token for a named profile from the keychain.
func ClearProfileToken(profileName string) error {
	kc := getKeychain()
	err := kc.Delete(keychainService, profileAccount(profileName))
	if err != nil && !isNotFoundErr(err) {
		return fmt.Errorf("failed to remove token from keychain: %w", err)
	}
	return nil
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
