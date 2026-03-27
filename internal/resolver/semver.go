package resolver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a parsed semver version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Raw        string
}

var semverRe = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-([a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*))?$`)

// ParseVersion parses a semver string.
func ParseVersion(s string) (*Version, error) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("invalid semver: %q", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: m[5],
		Raw:        s,
	}, nil
}

// String returns the canonical semver string.
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	return s
}

// Compare returns -1, 0, or 1.
func (v *Version) Compare(other *Version) int {
	if c := cmpInt(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmpInt(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmpInt(v.Patch, other.Patch); c != 0 {
		return c
	}
	// No prerelease > has prerelease
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// Constraint represents a version constraint like ^1.0.0 or ~1.2.
type Constraint struct {
	Op      string // "^", "~", "=", ">=", "<=", ">", "<", "*"
	Version *Version
	Raw     string
}

// ParseConstraint parses a version constraint string.
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" || s == "latest" {
		return &Constraint{Op: "*", Raw: s}, nil
	}

	for _, op := range []string{">=", "<=", "^", "~", ">", "<", "="} {
		if strings.HasPrefix(s, op) {
			v, err := ParseVersion(strings.TrimSpace(s[len(op):]))
			if err != nil {
				return nil, fmt.Errorf("parse constraint %q: %w", s, err)
			}
			return &Constraint{Op: op, Version: v, Raw: s}, nil
		}
	}

	// Exact version
	v, err := ParseVersion(s)
	if err != nil {
		return nil, fmt.Errorf("parse constraint %q: %w", s, err)
	}
	return &Constraint{Op: "=", Version: v, Raw: s}, nil
}

// Match checks if a version satisfies the constraint.
func (c *Constraint) Match(v *Version) bool {
	if c.Op == "*" {
		return v.Prerelease == "" // exclude prereleases from wildcard
	}

	switch c.Op {
	case "=":
		return v.Compare(c.Version) == 0
	case ">":
		return v.Compare(c.Version) > 0
	case ">=":
		return v.Compare(c.Version) >= 0
	case "<":
		return v.Compare(c.Version) < 0
	case "<=":
		return v.Compare(c.Version) <= 0
	case "^":
		// ^1.2.3 means >=1.2.3 <2.0.0 (same major)
		if v.Compare(c.Version) < 0 {
			return false
		}
		if c.Version.Major == 0 {
			// ^0.x.y is more restrictive: same minor
			return v.Major == 0 && v.Minor == c.Version.Minor
		}
		return v.Major == c.Version.Major
	case "~":
		// ~1.2.3 means >=1.2.3 <1.3.0 (same major.minor)
		if v.Compare(c.Version) < 0 {
			return false
		}
		return v.Major == c.Version.Major && v.Minor == c.Version.Minor
	}
	return false
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

// comparePrerelease compares prerelease identifiers per semver spec:
// numeric identifiers are compared as integers, string identifiers lexicographically.
func comparePrerelease(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}
	for i := 0; i < minLen; i++ {
		numA, errA := strconv.Atoi(partsA[i])
		numB, errB := strconv.Atoi(partsB[i])
		if errA == nil && errB == nil {
			if c := cmpInt(numA, numB); c != 0 {
				return c
			}
		} else if errA == nil {
			return -1 // numeric < string
		} else if errB == nil {
			return 1 // string > numeric
		} else {
			if c := strings.Compare(partsA[i], partsB[i]); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(partsA), len(partsB))
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
