// Package secrets provides a simple file-based store for MCP server
// environment variables (API keys, tokens, etc.).
//
// Secrets are stored in ~/.ctx/secrets.json with 0600 file permissions.
// Each package has its own namespace to avoid key collisions.
package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/ctx-hq/ctx/internal/config"
)

const secretsFile = "secrets.json"

// Store holds per-package environment variable secrets.
type Store struct {
	mu      sync.Mutex
	Secrets map[string]map[string]string `json:"secrets"` // fullName -> key -> value
}

// New creates an empty secrets store.
func New() *Store {
	return &Store{
		Secrets: make(map[string]map[string]string),
	}
}

// Load reads the secrets store from disk. Returns a new empty store if the
// file does not exist.
func Load() (*Store, error) {
	p := filePath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, err
	}

	s := New()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Secrets == nil {
		s.Secrets = make(map[string]map[string]string)
	}
	return s, nil
}

// Save writes the store to disk atomically with 0600 permissions.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := filePath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file then rename
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp) // clean up temp file containing secrets
		return err
	}
	return nil
}

// Set stores a secret for a package.
func (s *Store) Set(pkg, key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Secrets[pkg] == nil {
		s.Secrets[pkg] = make(map[string]string)
	}
	s.Secrets[pkg][key] = value
}

// Get retrieves a secret for a package.
func (s *Store) Get(pkg, key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.Secrets[pkg]
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	return v, ok
}

// Delete removes a secret for a package.
func (s *Store) Delete(pkg, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.Secrets[pkg]
	if m == nil {
		return
	}
	delete(m, key)
	if len(m) == 0 {
		delete(s.Secrets, pkg)
	}
}

// List returns all secrets for a package. Returns nil if the package has no secrets.
func (s *Store) List(pkg string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.Secrets[pkg]
	if m == nil {
		return nil
	}
	// Return a copy to avoid races
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// DeletePackage removes all secrets for a package.
func (s *Store) DeletePackage(pkg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Secrets, pkg)
}

func filePath() string {
	return filepath.Join(config.Dir(), secretsFile)
}
