package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
)

// isSingleFile checks if the given path is a .md file (not a directory).
func isSingleFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md")
}

// resolveBaseVersion reads the current version from an existing local skills dir,
// falling back to "0.1.0" if the package hasn't been published locally before.
func resolveBaseVersion(scope, skillName string) string {
	dest := filepath.Join(config.SkillsDir(), scope, skillName)
	existing, err := manifest.LoadFromDir(dest)
	if err == nil && existing.Version != "" {
		return existing.Version
	}
	return "0.1.0"
}

// linkToOriginal backs up the original file, creates a symlink to the skill's SKILL.md,
// and records the link in ~/.ctx/links.json.
// The operation is atomic: if symlink creation fails, the backup is restored.
func linkToOriginal(originalPath, targetPath, fullName string) error {
	// Check if original is already a symlink to target
	if link, err := os.Readlink(originalPath); err == nil && link == targetPath {
		return nil // already linked
	}

	// Backup original (if it exists and is not already a symlink to our target)
	bakPath := originalPath + ".bak"
	hadOriginal := false
	if _, err := os.Lstat(originalPath); err == nil {
		hadOriginal = true
		if err := os.Rename(originalPath, bakPath); err != nil {
			return fmt.Errorf("backup %s: %w", originalPath, err)
		}
	}

	// Create symlink — restore backup on failure
	if err := os.Symlink(targetPath, originalPath); err != nil {
		if hadOriginal {
			_ = os.Rename(bakPath, originalPath) // restore
		}
		return fmt.Errorf("symlink: %w", err)
	}

	// Record in links.json (including backup path for safe cleanup)
	links, err := installer.LoadLinks()
	if err != nil {
		return nil // non-fatal: link created, just couldn't record
	}
	entry := installer.LinkEntry{
		Agent:  "local",
		Type:   installer.LinkSymlink,
		Source: targetPath,
		Target: originalPath,
	}
	if hadOriginal {
		entry.ConfigKey = bakPath // store backup path for restore on cleanup
	}
	links.Add(fullName, entry)
	_ = links.Save() // non-fatal

	return nil
}

// extractDescription extracts a description from skill body content.
// Uses the first non-empty, non-heading line.
func extractDescription(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return truncate(line, 100)
	}
	return ""
}

// truncate shortens a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
