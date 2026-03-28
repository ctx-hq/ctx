package main

import (
	"fmt"
	"strings"
)

// parsePackageRef parses a scoped package reference like "@scope/name@version"
// into its fullName ("@scope/name") and version ("1.0.0") parts.
// Returns an error if the reference is not a valid scoped package with version.
func parsePackageRef(ref string) (fullName, version string, err error) {
	if !strings.HasPrefix(ref, "@") {
		return "", "", fmt.Errorf("invalid package reference: %s", ref)
	}

	rest := ref[1:]
	atIdx := strings.LastIndex(rest, "@")
	if atIdx == -1 {
		return "", "", fmt.Errorf("must specify version: %s@<version>", ref)
	}

	fullName = "@" + rest[:atIdx]
	version = rest[atIdx+1:]

	if version == "" {
		return "", "", fmt.Errorf("version is required")
	}

	// Validate @scope/name structure: must contain "/" with non-empty scope and name
	slashIdx := strings.Index(fullName[1:], "/")
	if slashIdx == -1 || slashIdx == 0 || slashIdx == len(fullName[1:])-1 {
		return "", "", fmt.Errorf("invalid package reference: expected @scope/name@version, got %s", ref)
	}

	return fullName, version, nil
}
