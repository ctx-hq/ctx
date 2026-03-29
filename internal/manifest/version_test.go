package manifest

import "testing"

func TestBumpPatch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0.0.0", "0.0.1"},
		{"0.1.0", "0.1.1"},
		{"1.2.3", "1.2.4"},
		{"1.0.0-beta.1", "1.0.1"},
		{"0.99.99", "0.99.100"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := BumpPatch(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("BumpPatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBumpMinor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0.0.0", "0.1.0"},
		{"1.2.3", "1.3.0"},
		{"1.0.0-rc.1", "1.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := BumpMinor(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("BumpMinor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBumpMajor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0.1.0", "1.0.0"},
		{"1.2.3", "2.0.0"},
		{"9.9.9", "10.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := BumpMajor(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("BumpMajor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBumpVersion_InvalidStrategy(t *testing.T) {
	_, err := BumpVersion("1.0.0", "huge")
	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

func TestBumpVersion_InvalidSemver(t *testing.T) {
	invalids := []string{"", "1.0", "v1.0.0", "abc", "1.2.3.4", "latest"}
	for _, v := range invalids {
		t.Run(v, func(t *testing.T) {
			_, err := BumpPatch(v)
			if err == nil {
				t.Errorf("BumpPatch(%q) should error for invalid semver", v)
			}
		})
	}
}

func TestBumpVersion_Dispatch(t *testing.T) {
	tests := []struct {
		version  string
		strategy string
		want     string
	}{
		{"1.0.0", "patch", "1.0.1"},
		{"1.0.0", "minor", "1.1.0"},
		{"1.0.0", "major", "2.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got, err := BumpVersion(tt.version, tt.strategy)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("BumpVersion(%q, %q) = %q, want %q", tt.version, tt.strategy, got, tt.want)
			}
		})
	}
}
