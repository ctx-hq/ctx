package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
)

// fileKeychain stores credentials in ~/.ctx/credentials with 0o600 permissions.
// Used as fallback when no platform keychain (macOS Keychain, Linux secret-tool) is available.
type fileKeychain struct{}

func credentialsPath() string {
	return filepath.Join(config.Dir(), "credentials")
}

func (k *fileKeychain) Get(service, account string) (string, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no credentials stored")
		}
		return "", fmt.Errorf("read credentials: %w", err)
	}

	key := service + ":" + account
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"="), nil
		}
	}
	return "", fmt.Errorf("credential not found: %s/%s", service, account)
}

func (k *fileKeychain) Set(service, account, secret string) error {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), config.DirPerm); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	key := service + ":" + account
	newLine := key + "=" + secret

	// Read existing, update or append
	var lines []string
	data, err := os.ReadFile(path)
	if err == nil {
		found := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, key+"=") {
				lines = append(lines, newLine)
				found = true
			} else if line != "" {
				lines = append(lines, line)
			}
		}
		if !found {
			lines = append(lines, newLine)
		}
	} else {
		lines = []string{newLine}
	}

	content := strings.Join(lines, "\n") + "\n"
	return atomicWriteFile(path, []byte(content), config.FilePerm)
}

func (k *fileKeychain) Delete(service, account string) error {
	path := credentialsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read credentials: %w", err)
	}

	key := service + ":" + account
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, key+"=") && line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return os.Remove(path)
	}

	content := strings.Join(lines, "\n") + "\n"
	return atomicWriteFile(path, []byte(content), config.FilePerm)
}

// atomicWriteFile writes data to a temp file and renames it to path,
// preventing partial writes from corrupting the credentials file.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
