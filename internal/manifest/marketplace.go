package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// MarketplaceFile represents the top-level .claude-plugin/marketplace.json structure.
type MarketplaceFile struct {
	Name     string              `json:"name"`
	Owner    *MarketplaceOwner   `json:"owner,omitempty"`
	Metadata *MarketplaceMeta    `json:"metadata,omitempty"`
	Plugins  []MarketplacePlugin `json:"plugins"`
}

// MarketplaceOwner identifies the marketplace package owner.
type MarketplaceOwner struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// MarketplaceMeta holds top-level marketplace metadata.
type MarketplaceMeta struct {
	Description string `json:"description"`
	Version     string `json:"version"`
}

// MarketplacePlugin represents a single plugin entry containing skill paths.
type MarketplacePlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Strict      bool     `json:"strict"`
	Skills      []string `json:"skills"` // paths like "./skills/baoyu-translate"
}

// ParseMarketplaceJSON parses a .claude-plugin/marketplace.json file.
func ParseMarketplaceJSON(r io.Reader) (*MarketplaceFile, error) {
	var mf MarketplaceFile
	if err := json.NewDecoder(r).Decode(&mf); err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}
	return &mf, nil
}

// MarketplaceSkillPaths returns deduplicated, cleaned skill directory paths
// from all plugins in a marketplace file. Leading "./" is stripped.
func MarketplaceSkillPaths(mf *MarketplaceFile) []string {
	seen := make(map[string]bool)
	var paths []string

	for _, plugin := range mf.Plugins {
		for _, raw := range plugin.Skills {
			// Normalize: strip leading "./" and trailing "/"
			p := strings.TrimPrefix(raw, "./")
			p = strings.TrimSuffix(p, "/")
			p = filepath.Clean(p)

			if p == "" || p == "." {
				continue
			}
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}

	return paths
}
