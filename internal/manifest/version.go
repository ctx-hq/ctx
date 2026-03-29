package manifest

import (
	"fmt"
	"strconv"
	"strings"
)

// BumpPatch increments the patch version: "1.2.3" → "1.2.4".
// Pre-release suffixes are stripped: "1.0.0-beta.1" → "1.0.1".
func BumpPatch(v string) (string, error) {
	major, minor, patch, _, err := parseSemver(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
}

// BumpMinor increments the minor version and resets patch: "1.2.3" → "1.3.0".
func BumpMinor(v string) (string, error) {
	major, minor, _, _, err := parseSemver(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.0", major, minor+1), nil
}

// BumpMajor increments the major version and resets minor+patch: "1.2.3" → "2.0.0".
func BumpMajor(v string) (string, error) {
	major, _, _, _, err := parseSemver(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.0.0", major+1), nil
}

// BumpVersion applies a named strategy ("patch", "minor", "major") to a version string.
func BumpVersion(v, strategy string) (string, error) {
	switch strategy {
	case "patch":
		return BumpPatch(v)
	case "minor":
		return BumpMinor(v)
	case "major":
		return BumpMajor(v)
	default:
		return "", fmt.Errorf("unknown bump strategy %q (use patch, minor, or major)", strategy)
	}
}

// parseSemver splits a version string into components.
// Accepts "major.minor.patch" and "major.minor.patch-prerelease".
func parseSemver(v string) (major, minor, patch int, pre string, err error) {
	if !semverRegex.MatchString(v) {
		return 0, 0, 0, "", fmt.Errorf("invalid semver %q", v)
	}

	// Split off pre-release
	core := v
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		core = v[:idx]
		pre = v[idx+1:]
	}

	parts := strings.SplitN(core, ".", 3)
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	patch, _ = strconv.Atoi(parts[2])
	return major, minor, patch, pre, nil
}
