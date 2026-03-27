package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const lockFileVersion = 1

// LockFile represents ctx.lock — tracks installed packages.
type LockFile struct {
	Version  int                    `json:"version"`
	Packages map[string]LockEntry   `json:"packages"` // keyed by full_name
}

// LockEntry tracks one installed package.
type LockEntry struct {
	FullName    string    `json:"full_name"`
	Version     string    `json:"version"`
	Type        string    `json:"type"`
	Source      string    `json:"source"`       // "registry", "github"
	SHA256      string    `json:"sha256,omitempty"`
	InstallPath string    `json:"install_path"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LoadLockFile reads ctx.lock from the given path.
func LoadLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{
				Version:  lockFileVersion,
				Packages: make(map[string]LockEntry),
			}, nil
		}
		return nil, fmt.Errorf("read lockfile: %w", err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Packages == nil {
		lf.Packages = make(map[string]LockEntry)
	}
	return &lf, nil
}

// Save writes the lockfile to disk.
func (lf *LockFile) Save(path string) error {
	lf.Version = lockFileVersion
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}
	return nil
}

// Add adds or updates a package in the lockfile.
func (lf *LockFile) Add(entry LockEntry) {
	now := time.Now().UTC()
	if existing, ok := lf.Packages[entry.FullName]; ok {
		entry.InstalledAt = existing.InstalledAt
		entry.UpdatedAt = now
	} else {
		entry.InstalledAt = now
		entry.UpdatedAt = now
	}
	lf.Packages[entry.FullName] = entry
}

// Remove removes a package from the lockfile.
func (lf *LockFile) Remove(fullName string) {
	delete(lf.Packages, fullName)
}

// Has checks if a package is in the lockfile.
func (lf *LockFile) Has(fullName string) bool {
	_, ok := lf.Packages[fullName]
	return ok
}

// Get returns a lock entry.
func (lf *LockFile) Get(fullName string) (LockEntry, bool) {
	e, ok := lf.Packages[fullName]
	return e, ok
}

// List returns all entries sorted by name.
func (lf *LockFile) List() []LockEntry {
	entries := make([]LockEntry, 0, len(lf.Packages))
	for _, e := range lf.Packages {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].FullName < entries[j].FullName
	})
	return entries
}
