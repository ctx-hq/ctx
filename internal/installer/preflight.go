package installer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// PreflightResult contains the result of a preflight check.
type PreflightResult struct {
	Passed  bool
	Missing []string // missing binary names
	Version map[string]string // detected versions for checked bins
	Errors  []string
}

// RunPreflight checks that runtime prerequisites declared in mcp.require are met.
// Returns nil result if there are no requirements to check.
func RunPreflight(m *manifest.Manifest) *PreflightResult {
	if m.MCP == nil || m.MCP.Require == nil {
		return nil
	}
	req := m.MCP.Require
	if len(req.Bins) == 0 && len(req.MinVersions) == 0 {
		return nil
	}

	result := &PreflightResult{
		Passed:  true,
		Version: make(map[string]string),
	}

	for _, bin := range req.Bins {
		path, err := exec.LookPath(bin)
		if err != nil {
			result.Passed = false
			result.Missing = append(result.Missing, bin)
			hint := installHint(bin)
			if minVer, ok := req.MinVersions[bin]; ok {
				result.Errors = append(result.Errors, fmt.Sprintf("%s (>= %s) is required but not found in PATH. %s", bin, minVer, hint))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("%s is required but not found in PATH. %s", bin, hint))
			}
			continue
		}
		_ = path

		// Check minimum version if specified
		if minVer, ok := req.MinVersions[bin]; ok {
			detected := detectVersion(bin)
			if detected != "" {
				result.Version[bin] = detected
				if !versionSatisfies(detected, minVer) {
					result.Passed = false
					result.Errors = append(result.Errors,
						fmt.Sprintf("%s version %s found, but >= %s is required", bin, detected, minVer))
				}
			}
		}
	}

	return result
}

// detectVersion runs "<bin> --version" and extracts the first semver-like string.
func detectVersion(bin string) string {
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return extractVersion(string(out))
}

// extractVersion finds the first semver-like version (X.Y.Z) in a string.
func extractVersion(s string) string {
	for _, word := range strings.Fields(s) {
		// Strip leading 'v'
		w := strings.TrimPrefix(word, "v")
		// Strip trailing punctuation
		w = strings.TrimRight(w, ",;:)")
		parts := strings.SplitN(w, ".", 3)
		if len(parts) >= 2 && isDigits(parts[0]) && isDigits(parts[1]) {
			return w
		}
	}
	return ""
}

// versionSatisfies checks if detected >= required using simple major.minor.patch comparison.
func versionSatisfies(detected, required string) bool {
	d := parseVersionParts(detected)
	r := parseVersionParts(required)
	for i := 0; i < 3; i++ {
		if d[i] > r[i] {
			return true
		}
		if d[i] < r[i] {
			return false
		}
	}
	return true // equal
}

func parseVersionParts(v string) [3]int {
	var parts [3]int
	// Strip prerelease suffix
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	for i, s := range strings.SplitN(v, ".", 3) {
		if i >= 3 {
			break
		}
		n := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				break
			}
		}
		parts[i] = n
	}
	return parts
}

// installHint returns a user-friendly install suggestion for common binaries.
func installHint(bin string) string {
	hints := map[string]string{
		"node":   "Install Node.js: https://nodejs.org/",
		"npm":    "Install Node.js (includes npm): https://nodejs.org/",
		"npx":    "Install Node.js (includes npx): https://nodejs.org/",
		"docker": "Install Docker: https://docs.docker.com/get-docker/",
		"python": "Install Python: https://www.python.org/downloads/",
		"pip":    "Install Python (includes pip): https://www.python.org/downloads/",
		"go":     "Install Go: https://go.dev/dl/",
		"cargo":  "Install Rust (includes cargo): https://rustup.rs/",
		"ruby":   "Install Ruby: https://www.ruby-lang.org/en/downloads/",
		"gem":    "Install Ruby (includes gem): https://www.ruby-lang.org/en/downloads/",
	}
	if hint, ok := hints[bin]; ok {
		return hint
	}
	return "Please install " + bin + " and ensure it is in your PATH"
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
