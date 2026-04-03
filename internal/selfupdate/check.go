package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/ctx-hq/ctx/internal/config"
)

// githubToken returns a GitHub token from environment for API auth.
// Checks GH_TOKEN (gh CLI convention) then GITHUB_TOKEN.
func githubToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}

const (
	checkInterval = 24 * time.Hour
	repo          = "ctx-hq/ctx"
	cacheFile     = "update-check.json"
)

// UpdateCache stores the last check time and result.
type UpdateCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

// CheckForUpdate checks GitHub for a newer version. Returns the latest version
// if newer than current, or empty string if up to date. Uses a 24h cache to
// avoid excessive API calls. This function never returns errors — it silently
// fails on network issues, missing cache, etc.
func CheckForUpdate(currentVersion string) string {
	if currentVersion == "" || currentVersion == "dev" {
		return ""
	}

	cachePath := filepath.Join(config.Dir(), cacheFile)

	// Check cache first
	cache, _ := loadCache(cachePath)
	if cache != nil && time.Since(cache.LastCheck) < checkInterval {
		if IsNewer(cache.LatestVersion, currentVersion) {
			return cache.LatestVersion
		}
		return ""
	}

	// Fetch latest version from GitHub (with short timeout)
	latest := fetchLatestVersion()
	if latest == "" {
		return ""
	}

	// Save cache
	saveCache(cachePath, &UpdateCache{
		LastCheck:     time.Now().UTC(),
		LatestVersion: latest,
	})

	if IsNewer(latest, currentVersion) {
		return latest
	}
	return ""
}

// FetchLatestVersion returns the latest version string from GitHub releases.
// Exported for use by the upgrade command.
func FetchLatestVersion() string {
	return fetchLatestVersion()
}

func fetchLatestVersion() string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", config.UserAgent())
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return ""
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}
	return strings.TrimPrefix(release.TagName, "v")
}

// IsNewer returns true if latest is a higher semver than current.
// Returns false if either version cannot be parsed as semver.
// Handles "v" prefixes transparently.
// Pre-release suffixes are stripped so that "0.4.0-beta" compares as "0.4.0"
// for upgrade-check purposes.
func IsNewer(latest, current string) bool {
	lv, cv := parseCoreVersion(latest), parseCoreVersion(current)
	if lv == nil || cv == nil {
		return false
	}
	return lv.GreaterThan(cv)
}

// IsUpToDate returns true only when both versions are valid semver and
// current >= latest. Returns false if either version cannot be parsed
// (e.g. "dev", empty), meaning the caller should NOT skip the upgrade.
func IsUpToDate(latest, current string) bool {
	lv, cv := parseCoreVersion(latest), parseCoreVersion(current)
	if lv == nil || cv == nil {
		return false
	}
	return !cv.LessThan(lv) // current >= latest
}

// parseCoreVersion parses a version string, stripping the pre-release suffix
// so that upgrade checks compare only major.minor.patch.
func parseCoreVersion(v string) *semver.Version {
	if v == "" {
		return nil
	}
	sv, err := semver.NewVersion(v)
	if err != nil {
		return nil
	}
	// Strip pre-release for upgrade comparison (e.g. 0.4.0-beta → 0.4.0).
	if sv.Prerelease() != "" {
		core, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d", sv.Major(), sv.Minor(), sv.Patch()))
		return core
	}
	return sv
}

func loadCache(path string) (*UpdateCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func saveCache(path string, cache *UpdateCache) {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0o700)
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
