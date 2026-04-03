package resolver

import (
	"fmt"
	"strings"

	libsemver "github.com/Masterminds/semver/v3"
)

// Version represents a parsed semver version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Raw        string
	inner      *libsemver.Version
}

// ParseVersion parses a semver string.
func ParseVersion(s string) (*Version, error) {
	v, err := libsemver.StrictNewVersion(strings.TrimPrefix(s, "v"))
	if err != nil {
		return nil, fmt.Errorf("invalid semver: %q", s)
	}
	return &Version{
		Major:      int(v.Major()),
		Minor:      int(v.Minor()),
		Patch:      int(v.Patch()),
		Prerelease: v.Prerelease(),
		Raw:        s,
		inner:      v,
	}, nil
}

// String returns the canonical semver string.
func (v *Version) String() string {
	return v.inner.String()
}

// Compare returns -1, 0, or 1.
func (v *Version) Compare(other *Version) int {
	return v.inner.Compare(other.inner)
}

// Constraint represents a version constraint like ^1.0.0 or ~1.2.
type Constraint struct {
	Op      string // "^", "~", "=", ">=", "<=", ">", "<", "*"
	Version *Version
	Raw     string
	inner   *libsemver.Constraints
}

// ParseConstraint parses a version constraint string.
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" || s == "latest" {
		return &Constraint{Op: "*", Raw: s}, nil
	}

	// Determine the operator and version string.
	op := "="
	vStr := s
	for _, candidate := range []string{">=", "<=", "^", "~", ">", "<", "="} {
		if strings.HasPrefix(s, candidate) {
			op = candidate
			vStr = strings.TrimSpace(s[len(candidate):])
			break
		}
	}
	ver, err := ParseVersion(vStr)
	if err != nil {
		return nil, fmt.Errorf("parse constraint %q: %w", s, err)
	}

	// Build the library constraint string.
	// The Masterminds library uses different syntax for some operators.
	libStr := s
	// Ensure no 'v' prefix on the version portion for the library.
	libStr = op + " " + strings.TrimPrefix(vStr, "v")

	inner, err := libsemver.NewConstraint(libStr)
	if err != nil {
		return nil, fmt.Errorf("parse constraint %q: %w", s, err)
	}

	return &Constraint{Op: op, Version: ver, Raw: s, inner: inner}, nil
}

// Match checks if a version satisfies the constraint.
func (c *Constraint) Match(v *Version) bool {
	if c.Op == "*" {
		return v.Prerelease == "" // exclude prereleases from wildcard
	}
	return c.inner.Check(v.inner)
}

// BestMatch finds the highest version matching the constraint.
func BestMatch(versions []*Version, constraint *Constraint) *Version {
	var best *Version
	for _, v := range versions {
		if !constraint.Match(v) {
			continue
		}
		if best == nil || v.Compare(best) > 0 {
			best = v
		}
	}
	return best
}
