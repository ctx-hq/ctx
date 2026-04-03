package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	semver "github.com/Masterminds/semver/v3"
)

// VersionDir returns the versioned install path: ~/.ctx/packages/@scope/name/{version}/
func (i *Installer) VersionDir(fullName, version string) string {
	return filepath.Join(i.DataDir, fullName, version)
}

// PackageDir returns the package root: ~/.ctx/packages/@scope/name/
func (i *Installer) PackageDir(fullName string) string {
	return filepath.Join(i.DataDir, fullName)
}

// CurrentLink returns the path to the current symlink: ~/.ctx/packages/@scope/name/current
func (i *Installer) CurrentLink(fullName string) string {
	return filepath.Join(i.PackageDir(fullName), "current")
}

// CurrentVersion reads the current symlink and returns the version it points to.
// Returns empty string if no current symlink exists.
func (i *Installer) CurrentVersion(fullName string) string {
	link := i.CurrentLink(fullName)
	dest, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	// dest is a relative path like "1.0.0"
	return filepath.Base(dest)
}

// InstalledVersions returns all locally installed versions for a package, sorted.
func (i *Installer) InstalledVersions(fullName string) []string {
	pkgDir := i.PackageDir(fullName)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "current" && !isHidden(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) < 0
	})
	return versions
}

// compareSemver compares two version strings.
// Valid semver versions always sort after non-semver strings (e.g. branch names
// like "main") so that PruneVersions keeps real releases and discards refs first.
// Within each group the comparison is numeric (semver) or lexicographic (non-semver).
func compareSemver(a, b string) int {
	va, errA := semver.NewVersion(a)
	vb, errB := semver.NewVersion(b)

	switch {
	case errA == nil && errB == nil:
		return va.Compare(vb)
	case errA == nil:
		// a is semver, b is not → a sorts after b
		return 1
	case errB == nil:
		// b is semver, a is not → a sorts before b
		return -1
	default:
		// Neither is semver — lexicographic.
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
}

// SwitchCurrent atomically switches the `current` symlink to point to newVersion.
// Uses tmp symlink + os.Rename for POSIX atomicity.
func SwitchCurrent(pkgDir, newVersion string) error {
	current := filepath.Join(pkgDir, "current")

	// Verify target version directory exists
	versionDir := filepath.Join(pkgDir, newVersion)
	if _, err := os.Stat(versionDir); err != nil {
		return fmt.Errorf("version directory %q does not exist: %w", newVersion, err)
	}

	// Create temp symlink with unique name
	tmp := current + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 36)
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temp symlink: %w", err)
	}

	// Symlink to relative path (just the version directory name)
	if err := os.Symlink(newVersion, tmp); err != nil {
		return fmt.Errorf("create temp symlink: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmp, current); err != nil {
		_ = os.Remove(tmp) // cleanup on failure
		return fmt.Errorf("atomic switch: %w", err)
	}

	return nil
}

// PruneVersions removes old versions, keeping the specified count of most recent.
// Always keeps the current version regardless of keepCount.
// Returns list of removed versions and total bytes freed.
func (i *Installer) PruneVersions(fullName string, keepCount int) ([]string, int64, error) {
	if keepCount < 1 {
		keepCount = 1
	}

	current := i.CurrentVersion(fullName)
	versions := i.InstalledVersions(fullName)

	if len(versions) == 0 {
		return nil, 0, nil
	}

	// Determine which versions to keep
	keep := make(map[string]bool)
	if current != "" {
		keep[current] = true
	}

	// Keep the most recent N versions (versions are sorted)
	for idx := len(versions) - 1; idx >= 0 && len(keep) < keepCount; idx-- {
		keep[versions[idx]] = true
	}

	// Remove non-kept versions
	var removed []string
	var freed int64

	for _, v := range versions {
		if keep[v] {
			continue
		}
		vDir := filepath.Join(i.PackageDir(fullName), v)
		size := dirSize(vDir)
		if err := os.RemoveAll(vDir); err != nil {
			continue // skip failures silently
		}
		removed = append(removed, v)
		freed += size
	}

	return removed, freed, nil
}

// isHidden returns true if a name starts with '.'
func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

// dirSize recursively calculates directory size in bytes.
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
