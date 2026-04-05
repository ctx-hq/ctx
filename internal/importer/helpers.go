// Package importer detects skill repo layouts and extracts skill metadata.
// It supports 7 format types: marketplace.json, codex, single-skill, flat,
// nested, bare markdown, and unknown.
package importer

import (
	"os"
	"regexp"
	"strings"
	"unicode"
)

var slugMultiDash = regexp.MustCompile(`-{2,}`)

// Slugify converts a string to a lowercase, hyphen-separated slug.
func Slugify(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			return r
		}
		return '-'
	}, s)
	s = slugMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// FileExistsAt checks if a regular file exists at path.
func FileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DirExistsAt checks if a directory exists at path.
func DirExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ExcludedDirs are directories that should never be treated as skill packages.
var ExcludedDirs = map[string]bool{
	"internal":     true,
	"cmd":          true,
	"pkg":          true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true,
	"__pycache__":  true,
	"docs":         true,
	"test":         true,
	"tests":        true,
	"fixtures":     true,
	"testdata":     true,
	"examples":     true,
	".github":      true,
	".claude":      true,
	".vscode":      true,
}

// ContainsExcludedDir checks if any path component is an excluded directory.
func ContainsExcludedDir(relPath string) bool {
	for _, part := range strings.Split(strings.ReplaceAll(relPath, "\\", "/"), "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
		if ExcludedDirs[part] {
			return true
		}
	}
	return false
}
