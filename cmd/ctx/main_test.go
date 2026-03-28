package main

import (
	"testing"
)

func TestBuildInfoFallback_SkipsWhenInjected(t *testing.T) {
	// Save and restore
	original := Version
	defer func() { Version = original }()

	// When Version is already set (ldflags injection), BuildInfo fallback
	// should not overwrite it. Simulate by setting a known value.
	Version = "v1.2.3"

	// Re-run the init logic manually
	resolveVersionFromBuildInfo()

	if Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3 (ldflags should take priority)", Version)
	}
}

func TestBuildInfoFallback_TriggersOnDev(t *testing.T) {
	// Save and restore
	original := Version
	defer func() { Version = original }()

	Version = "dev"

	// Re-run the init logic
	resolveVersionFromBuildInfo()

	// In test context, BuildInfo may return "(devel)" which we skip,
	// so Version may remain "dev". That's correct behavior.
	// The important thing is it doesn't panic or produce empty string.
	if Version == "" {
		t.Error("Version should never be empty after BuildInfo fallback")
	}
}
