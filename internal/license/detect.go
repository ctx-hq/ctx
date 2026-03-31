package license

import (
	"os"
	"path/filepath"
	"strings"
)

// Result holds the detected license information.
type Result struct {
	SPDX     string // SPDX identifier (e.g. "MIT", "Apache-2.0"), empty if unknown
	FilePath string // relative path to the license file (e.g. "LICENSE")
}

// licenseFiles lists candidate filenames in priority order.
var licenseFiles = []string{
	"LICENSE", "LICENSE.md", "LICENSE.txt",
	"LICENCE", "LICENCE.md", "LICENCE.txt",
	"COPYING", "COPYING.md", "COPYING.txt",
}

// Detect searches dir for a license file and attempts to identify its SPDX
// identifier. Returns a zero Result if no license file is found.
func Detect(dir string) Result {
	filePath := findLicenseFile(dir)
	if filePath == "" {
		return Result{}
	}

	data, err := os.ReadFile(filepath.Join(dir, filePath))
	if err != nil {
		return Result{FilePath: filePath}
	}

	spdx := identifySPDX(string(data))
	return Result{SPDX: spdx, FilePath: filePath}
}

// findLicenseFile searches dir for a license file, matching case-insensitively
// against the priority list. Returns the actual filename (preserving case) or "".
func findLicenseFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Build a map of lowercase → actual filename
	byLower := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			byLower[strings.ToLower(e.Name())] = e.Name()
		}
	}

	// Check candidates in priority order
	for _, candidate := range licenseFiles {
		if actual, ok := byLower[strings.ToLower(candidate)]; ok {
			return actual
		}
	}
	return ""
}

// identifySPDX attempts to identify the SPDX license identifier from file content.
// Returns "" if the license cannot be identified.
func identifySPDX(content string) string {
	// Fast path: check for SPDX header
	if spdx := extractSPDXHeader(content); spdx != "" {
		return spdx
	}

	lower := strings.ToLower(content)

	// Order matters: check more specific licenses before generic ones.
	switch {
	case strings.Contains(lower, "mit license") ||
		strings.Contains(lower, "permission is hereby granted, free of charge"):
		return "MIT"

	case strings.Contains(lower, "apache license") && strings.Contains(lower, "version 2.0"):
		return "Apache-2.0"

	case strings.Contains(lower, "gnu lesser general public license") && strings.Contains(lower, "version 2.1"):
		return "LGPL-2.1-only"

	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 3"):
		return "GPL-3.0-only"

	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 2"):
		return "GPL-2.0-only"

	case strings.Contains(lower, "mozilla public license") && strings.Contains(lower, "2.0"):
		return "MPL-2.0"

	case strings.Contains(lower, "this is free and unencumbered software released into the public domain"):
		return "Unlicense"

	case isBSD(lower):
		if strings.Contains(lower, "neither the name") || strings.Contains(lower, "the name of the") {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"

	case strings.Contains(lower, "isc license"):
		return "ISC"

	default:
		return ""
	}
}

// isBSD checks for BSD-style license markers.
func isBSD(lower string) bool {
	return (strings.Contains(lower, "bsd") || strings.Contains(lower, "redistribution and use in source and binary")) &&
		strings.Contains(lower, "redistribution")
}

// extractSPDXHeader looks for an SPDX-License-Identifier header line.
func extractSPDXHeader(content string) string {
	const prefix = "SPDX-License-Identifier:"
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Strip comment markers
		line = strings.TrimLeft(line, "/#*- ")
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
