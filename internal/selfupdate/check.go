package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
)

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
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", config.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
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
func IsNewer(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}
	lp := parseSemver(latest)
	cp := parseSemver(current)
	if lp == nil || cp == nil {
		return false
	}
	if lp[0] != cp[0] {
		return lp[0] > cp[0]
	}
	if lp[1] != cp[1] {
		return lp[1] > cp[1]
	}
	return lp[2] > cp[2]
}

// IsUpToDate returns true only when both versions are valid semver and
// current >= latest. Returns false if either version cannot be parsed
// (e.g. "dev", empty), meaning the caller should NOT skip the upgrade.
func IsUpToDate(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}
	lp := parseSemver(latest)
	cp := parseSemver(current)
	if lp == nil || cp == nil {
		return false // unparseable → not confidently up to date
	}
	// current >= latest
	if cp[0] != lp[0] {
		return cp[0] > lp[0]
	}
	if cp[1] != lp[1] {
		return cp[1] > lp[1]
	}
	return cp[2] >= lp[2]
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	result := make([]int, 3)
	for i, p := range parts {
		// Handle pre-release suffixes like "1.0.0-beta"
		p = strings.SplitN(p, "-", 2)[0]
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return nil
			}
			n = n*10 + int(c-'0')
		}
		result[i] = n
	}
	return result
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
	os.MkdirAll(dir, 0o700)
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0o600)
}
