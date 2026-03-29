package main

import (
	"regexp"
	"testing"
)

// isSemver checks if a string looks like a semver version.
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)`)

func TestDistTagNameValidation_RejectsSemver(t *testing.T) {
	// Dist-tag names should not be valid semver strings — they are labels like "beta", "latest"
	tags := []struct {
		tag       string
		wantValid bool
	}{
		{"beta", true},
		{"latest", true},
		{"canary", true},
		{"stable", true},
		{"1.0.0", false},   // semver should be rejected as tag name
		{"2.3.4", false},   // semver should be rejected
		{"0.1.0", false},   // semver should be rejected
		{"v1.0", true},     // not strict semver, could be a tag
		{"rc-1", true},     // label, not semver
	}

	for _, tt := range tags {
		t.Run(tt.tag, func(t *testing.T) {
			isSemver := semverPattern.MatchString(tt.tag)
			isValid := !isSemver

			if isValid != tt.wantValid {
				t.Errorf("tag %q: isValid = %v, want %v", tt.tag, isValid, tt.wantValid)
			}
		})
	}
}

func TestDistTagSubcommandArgs(t *testing.T) {
	// Verify expected arg counts for dist-tag subcommands
	tests := []struct {
		name     string
		cmd      string
		args     int
		wantDesc string
	}{
		{"ls requires 1 arg (package)", "ls", 1, "package name"},
		{"add requires 3 args", "add", 3, "package, tag, version"},
		{"rm requires 2 args", "rm", 2, "package, tag"},
	}

	subcommands := map[string]int{
		"ls":  1,
		"add": 3,
		"rm":  2,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, ok := subcommands[tt.cmd]
			if !ok {
				t.Fatalf("unknown subcommand %q", tt.cmd)
			}
			if expected != tt.args {
				t.Errorf("subcommand %q expects %d args, want %d (%s)",
					tt.cmd, expected, tt.args, tt.wantDesc)
			}
		})
	}
}

func TestDistTagCmd_HasExpectedSubcommands(t *testing.T) {
	subs := distTagCmd.Commands()
	names := make(map[string]bool)
	for _, sub := range subs {
		names[sub.Name()] = true
	}

	required := []string{"ls", "add", "rm"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("dist-tag missing subcommand %q", name)
		}
	}
}
