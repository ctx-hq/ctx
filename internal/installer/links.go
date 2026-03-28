package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/config"
)

// LinkType represents the type of external link ctx manages.
type LinkType string

const (
	LinkSymlink LinkType = "symlink" // Skill files linked to agent dirs
	LinkConfig  LinkType = "config"  // MCP config entries in agent JSON files
	LinkBinary  LinkType = "binary"  // CLI binary symlinks
)

const linksFileVersion = 1

// LinkEntry records a single link ctx created on the system.
type LinkEntry struct {
	Agent     string   `json:"agent"`
	Type      LinkType `json:"type"`
	Source    string   `json:"source"`               // SSOT path (what we link FROM)
	Target    string   `json:"target"`               // Agent-side path (what we link TO)
	ConfigKey string   `json:"config_key,omitempty"` // For MCP: key in mcpServers
	CreatedAt time.Time `json:"created_at"`
}

// LinkRegistry tracks all external modifications ctx has made.
type LinkRegistry struct {
	Version int                    `json:"version"`
	Links   map[string][]LinkEntry `json:"links"` // keyed by package fullName
}

// LinksFilePath returns the path to links.json.
func LinksFilePath() string {
	return filepath.Join(config.Dir(), "links.json")
}

// LoadLinks reads ~/.ctx/links.json (creates empty if missing).
func LoadLinks() (*LinkRegistry, error) {
	path := LinksFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LinkRegistry{
				Version: linksFileVersion,
				Links:   make(map[string][]LinkEntry),
			}, nil
		}
		return nil, fmt.Errorf("read links.json: %w", err)
	}

	var reg LinkRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse links.json: %w", err)
	}
	if reg.Links == nil {
		reg.Links = make(map[string][]LinkEntry)
	}
	reg.expandPaths()
	return &reg, nil
}

// Save writes the registry to ~/.ctx/links.json atomically (tmp + rename).
func (r *LinkRegistry) Save() error {
	r.Version = linksFileVersion
	path := LinksFilePath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create links dir: %w", err)
	}

	compressed := r.compressedCopy()
	data, err := json.MarshalIndent(compressed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal links.json: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".links-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp to links.json: %w", err)
	}
	return nil
}

// Add registers a new link for a package.
func (r *LinkRegistry) Add(fullName string, entry LinkEntry) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	// Avoid duplicates (same target)
	existing := r.Links[fullName]
	for i, e := range existing {
		if e.Target == entry.Target && e.Agent == entry.Agent {
			existing[i] = entry // update in place
			r.Links[fullName] = existing
			return
		}
	}
	r.Links[fullName] = append(existing, entry)
}

// Remove deletes all links for a package and returns them for cleanup.
func (r *LinkRegistry) Remove(fullName string) []LinkEntry {
	entries := r.Links[fullName]
	delete(r.Links, fullName)
	return entries
}

// ForPackage returns all links for a given package.
func (r *LinkRegistry) ForPackage(fullName string) []LinkEntry {
	return r.Links[fullName]
}

// LinkIssue describes a problem found during verification.
type LinkIssue struct {
	Package string    `json:"package"`
	Entry   LinkEntry `json:"entry"`
	Problem string    `json:"problem"`
}

// Verify checks all registered links and reports issues.
func (r *LinkRegistry) Verify() []LinkIssue {
	var issues []LinkIssue

	for pkg, entries := range r.Links {
		for _, entry := range entries {
			switch entry.Type {
			case LinkSymlink:
				// Check if symlink target exists and is a symlink
				fi, err := os.Lstat(entry.Target)
				if err != nil {
					issues = append(issues, LinkIssue{
						Package: pkg,
						Entry:   entry,
						Problem: "missing_target",
					})
					continue
				}
				if fi.Mode()&os.ModeSymlink == 0 {
					// Target exists but is not a symlink (maybe was replaced by copy)
					// Check if .ctx-managed marker exists
					markerPath := filepath.Join(entry.Target, ".ctx-managed")
					if _, err := os.Stat(markerPath); err != nil {
						issues = append(issues, LinkIssue{
							Package: pkg,
							Entry:   entry,
							Problem: "not_a_symlink",
						})
					}
					continue
				}
				// Check if symlink points to valid target
				dest, err := os.Readlink(entry.Target)
				if err != nil {
					issues = append(issues, LinkIssue{
						Package: pkg,
						Entry:   entry,
						Problem: "broken_symlink",
					})
					continue
				}
				// Resolve relative to parent dir for relative symlinks
				if !filepath.IsAbs(dest) {
					dest = filepath.Join(filepath.Dir(entry.Target), dest)
				}
				if _, err := os.Stat(dest); err != nil {
					issues = append(issues, LinkIssue{
						Package: pkg,
						Entry:   entry,
						Problem: "broken_symlink",
					})
				}

			case LinkConfig:
				// Check if the config file exists
				if _, err := os.Stat(entry.Target); err != nil {
					issues = append(issues, LinkIssue{
						Package: pkg,
						Entry:   entry,
						Problem: "missing_config_file",
					})
				}
				// Could also check if the key still exists in the JSON,
				// but that's more complex and can be added later
			}
		}
	}

	return issues
}

// CleanupLinks removes all system modifications tracked by link entries.
// Returns the count of successfully cleaned items.
func CleanupLinks(entries []LinkEntry) int {
	cleaned := 0
	for _, entry := range entries {
		switch entry.Type {
		case LinkSymlink:
			// Remove symlink or copied directory
			fi, err := os.Lstat(entry.Target)
			if err != nil {
				continue // already gone
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				os.Remove(entry.Target)
				cleaned++
			} else if fi.IsDir() {
				// Check for .ctx-managed marker before removing
				markerPath := filepath.Join(entry.Target, ".ctx-managed")
				if _, err := os.Stat(markerPath); err == nil {
					os.RemoveAll(entry.Target)
					cleaned++
				}
			}
			// Clean up empty parent directories
			cleanEmptyParents(filepath.Dir(entry.Target))

		case LinkConfig:
			if entry.ConfigKey != "" && entry.Agent != "" {
				a, err := agent.FindByName(entry.Agent)
				if err == nil {
					if err := a.RemoveMCP(entry.ConfigKey); err == nil {
						cleaned++
					}
				}
			}

		case LinkBinary:
			if _, err := os.Lstat(entry.Target); err == nil {
				os.Remove(entry.Target)
				cleaned++
			}
		}
	}
	return cleaned
}

// compressedCopy returns a copy with home directory paths replaced by "~".
func (r *LinkRegistry) compressedCopy() *LinkRegistry {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return r
	}
	homePrefix := home + string(filepath.Separator)

	clone := &LinkRegistry{
		Version: r.Version,
		Links:   make(map[string][]LinkEntry, len(r.Links)),
	}
	for k, entries := range r.Links {
		compressed := make([]LinkEntry, len(entries))
		for i, e := range entries {
			e.Source = compressPath(e.Source, home, homePrefix)
			e.Target = compressPath(e.Target, home, homePrefix)
			compressed[i] = e
		}
		clone.Links[k] = compressed
	}
	return clone
}

func compressPath(p, home, homePrefix string) string {
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, homePrefix) {
		return filepath.Join("~", strings.TrimPrefix(p, homePrefix))
	}
	return p
}

// expandPaths replaces "~" prefixed paths with the actual home directory.
func (r *LinkRegistry) expandPaths() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	for k, entries := range r.Links {
		for i, e := range entries {
			entries[i].Source = expandPath(e.Source, home)
			entries[i].Target = expandPath(e.Target, home)
		}
		r.Links[k] = entries
	}
}

func expandPath(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		return filepath.Join(home, strings.TrimPrefix(p, "~"+string(filepath.Separator)))
	}
	return p
}

// cleanEmptyParents removes empty parent directories up to a reasonable depth.
func cleanEmptyParents(dir string) {
	for i := 0; i < 3; i++ { // max 3 levels up
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
