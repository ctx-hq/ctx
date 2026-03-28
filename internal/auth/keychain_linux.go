//go:build linux

package auth

import (
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	if _, err := exec.LookPath("secret-tool"); err == nil {
		defaultKeychain = &linuxKeychain{}
	}
}

type linuxKeychain struct{}

func (k *linuxKeychain) Get(service, account string) (string, error) {
	out, err := exec.Command("secret-tool", "lookup",
		"service", service, "account", account).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("secret-tool lookup: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (k *linuxKeychain) Set(service, account, secret string) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", fmt.Sprintf("%s token", service),
		"service", service, "account", account)
	cmd.Stdin = strings.NewReader(secret)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool store: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (k *linuxKeychain) Delete(service, account string) error {
	out, err := exec.Command("secret-tool", "clear",
		"service", service, "account", account).CombinedOutput()
	if err != nil {
		// Ignore "not found" type errors
		if strings.Contains(string(out), "not found") {
			return nil
		}
		return fmt.Errorf("secret-tool clear: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
