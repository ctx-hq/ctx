package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ctx-hq/ctx/internal/config"
	"gopkg.in/yaml.v3"
)

// Profile represents a named identity + registry pair.
type Profile struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`
}

// RegistryURL returns the registry URL for this profile.
// Falls back to config.DefaultRegistry if not set.
func (p *Profile) RegistryURL() string {
	if p.Registry != "" {
		return p.Registry
	}
	return config.DefaultRegistry
}

// ProfileStore is the on-disk format of ~/.ctx/profiles.yaml.
type ProfileStore struct {
	Active   string              `yaml:"active,omitempty" json:"active,omitempty"`
	Profiles map[string]*Profile `yaml:"profiles" json:"profiles"`
}

// namePattern validates profile names: lowercase alphanumeric and hyphens,
// must start and end with alphanumeric, 1-64 characters.
var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

// ValidateName checks if a profile name is valid.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("profile name too long (max 64 characters)")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must be lowercase alphanumeric with hyphens, starting and ending with a letter or digit", name)
	}
	return nil
}

// profilesPath returns the path to ~/.ctx/profiles.yaml.
func profilesPath() string {
	return filepath.Join(config.Dir(), "profiles.yaml")
}

// Load reads the profile store from disk.
// Returns an empty store if profiles.yaml does not exist.
func Load() (*ProfileStore, error) {
	path := profilesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProfileStore{Profiles: make(map[string]*Profile)}, nil
		}
		return nil, fmt.Errorf("read profiles: %w", err)
	}

	store := &ProfileStore{
		Profiles: make(map[string]*Profile),
	}
	if err := yaml.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]*Profile)
	}
	return store, nil
}

// Save writes the profile store to disk atomically.
func (s *ProfileStore) Save() error {
	path := profilesPath()
	if err := os.MkdirAll(filepath.Dir(path), config.DirPerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}
	return atomicWriteFile(path, data, config.FilePerm)
}

// atomicWriteFile writes data to a temp file and renames it to path,
// preventing partial writes from corrupting the file.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
