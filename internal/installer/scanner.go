package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getctx/ctx/internal/manifest"
)

// InstalledPackage represents a package found on the filesystem.
type InstalledPackage struct {
	FullName    string `json:"full_name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	InstallPath string `json:"install_path"`
	Description string `json:"description,omitempty"`
}

// ScanInstalled walks ~/.ctx/packages/ and returns all installed packages
// by reading current/manifest.json for each package directory.
func (i *Installer) ScanInstalled() ([]InstalledPackage, error) {
	packagesDir := i.DataDir
	var result []InstalledPackage

	// Walk @scope directories
	topEntries, err := os.ReadDir(packagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read packages dir: %w", err)
	}

	for _, scopeEntry := range topEntries {
		if !scopeEntry.IsDir() {
			continue
		}
		scopeName := scopeEntry.Name()
		if !strings.HasPrefix(scopeName, "@") {
			continue // skip non-scope directories
		}

		scopeDir := filepath.Join(packagesDir, scopeName)
		pkgEntries, err := os.ReadDir(scopeDir)
		if err != nil {
			continue
		}

		for _, pkgEntry := range pkgEntries {
			if !pkgEntry.IsDir() {
				continue
			}
			fullName := scopeName + "/" + pkgEntry.Name()
			pkg, err := i.readInstalledPackage(fullName)
			if err != nil {
				continue // skip corrupt packages
			}
			result = append(result, *pkg)
		}
	}

	sort.Slice(result, func(a, b int) bool {
		return result[a].FullName < result[b].FullName
	})
	return result, nil
}

// IsInstalled checks if a package is installed by verifying the current symlink
// and manifest.json exist.
func (i *Installer) IsInstalled(fullName string) bool {
	manifestPath := filepath.Join(i.CurrentLink(fullName), "manifest.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}

// GetInstalled reads a single installed package's info from the filesystem.
func (i *Installer) GetInstalled(fullName string) (*InstalledPackage, error) {
	pkg, err := i.readInstalledPackage(fullName)
	if err != nil {
		return nil, fmt.Errorf("package %s is not installed", fullName)
	}
	return pkg, nil
}

// readInstalledPackage reads manifest.json from the current symlink of a package.
func (i *Installer) readInstalledPackage(fullName string) (*InstalledPackage, error) {
	currentPath := i.CurrentLink(fullName)
	manifestPath := filepath.Join(currentPath, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	version := i.CurrentVersion(fullName)

	return &InstalledPackage{
		FullName:    fullName,
		Version:     version,
		Type:        string(m.Type),
		InstallPath: currentPath,
		Description: m.Description,
	}, nil
}
