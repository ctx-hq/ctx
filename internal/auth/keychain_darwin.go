//go:build darwin

package auth

import (
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	if _, err := exec.LookPath("security"); err == nil {
		defaultKeychain = &darwinKeychain{}
	}
}

type darwinKeychain struct{}

func (k *darwinKeychain) Get(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keychain get: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (k *darwinKeychain) Set(service, account, secret string) error {
	// -U flag updates if exists, adds if not
	out, err := exec.Command("security", "add-generic-password",
		"-s", service, "-a", account, "-w", secret, "-U").CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain set: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (k *darwinKeychain) Delete(service, account string) error {
	out, err := exec.Command("security", "delete-generic-password",
		"-s", service, "-a", account).CombinedOutput()
	if err != nil {
		// Ignore "item not found" errors
		if strings.Contains(string(out), "could not be found") {
			return nil
		}
		return fmt.Errorf("keychain delete: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
