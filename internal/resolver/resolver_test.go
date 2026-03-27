package resolver

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		major   int
		minor   int
		patch   int
		pre     string
	}{
		{"1.0.0", false, 1, 0, 0, ""},
		{"0.1.0", false, 0, 1, 0, ""},
		{"14.1.0", false, 14, 1, 0, ""},
		{"1.0.0-beta.1", false, 1, 0, 0, "beta.1"},
		{"v2.3.4", false, 2, 3, 4, ""},
		{"invalid", true, 0, 0, 0, ""},
		{"1.0", true, 0, 0, 0, ""},
	}
	for _, tt := range tests {
		v, err := ParseVersion(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q) error: %v", tt.input, err)
			continue
		}
		if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch || v.Prerelease != tt.pre {
			t.Errorf("ParseVersion(%q) = %d.%d.%d-%s, want %d.%d.%d-%s",
				tt.input, v.Major, v.Minor, v.Patch, v.Prerelease,
				tt.major, tt.minor, tt.patch, tt.pre)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0-beta", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}
	for _, tt := range tests {
		a, _ := ParseVersion(tt.a)
		b, _ := ParseVersion(tt.b)
		got := a.Compare(b)
		if got != tt.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestConstraintMatch(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		{"^1.0.0", "1.0.0", true},
		{"^1.0.0", "1.9.9", true},
		{"^1.0.0", "2.0.0", false},
		{"^1.0.0", "0.9.0", false},
		{"~1.2.0", "1.2.5", true},
		{"~1.2.0", "1.3.0", false},
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "2.0.0", true},
		{">=1.0.0", "0.9.0", false},
		{"<2.0.0", "1.9.9", true},
		{"<2.0.0", "2.0.0", false},
		{"=1.0.0", "1.0.0", true},
		{"=1.0.0", "1.0.1", false},
		{"*", "1.0.0", true},
		{"*", "1.0.0-beta", false},
		{"^0.1.0", "0.1.5", true},
		{"^0.1.0", "0.2.0", false},
	}
	for _, tt := range tests {
		c, err := ParseConstraint(tt.constraint)
		if err != nil {
			t.Errorf("ParseConstraint(%q) error: %v", tt.constraint, err)
			continue
		}
		v, _ := ParseVersion(tt.version)
		got := c.Match(v)
		if got != tt.want {
			t.Errorf("Constraint(%q).Match(%q) = %v, want %v", tt.constraint, tt.version, got, tt.want)
		}
	}
}

func TestBestMatch(t *testing.T) {
	versions := make([]*Version, 0)
	for _, s := range []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0"} {
		v, _ := ParseVersion(s)
		versions = append(versions, v)
	}

	c, _ := ParseConstraint("^1.0.0")
	best := BestMatch(versions, c)
	if best == nil || best.String() != "1.2.0" {
		t.Errorf("BestMatch(^1.0.0) = %v, want 1.2.0", best)
	}

	c2, _ := ParseConstraint("~1.1.0")
	best2 := BestMatch(versions, c2)
	if best2 == nil || best2.String() != "1.1.0" {
		t.Errorf("BestMatch(~1.1.0) = %v, want 1.1.0", best2)
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		input          string
		wantName       string
		wantConstraint string
	}{
		{"@hong/my-skill", "@hong/my-skill", "*"},
		{"@hong/my-skill@^1.0.0", "@hong/my-skill", "^1.0.0"},
		{"@hong/my-skill@1.2.3", "@hong/my-skill", "1.2.3"},
		{"github:user/repo", "github:user/repo", "*"},
	}
	for _, tt := range tests {
		name, constraint := parseRef(tt.input)
		if name != tt.wantName || constraint != tt.wantConstraint {
			t.Errorf("parseRef(%q) = (%q, %q), want (%q, %q)",
				tt.input, name, constraint, tt.wantName, tt.wantConstraint)
		}
	}
}
