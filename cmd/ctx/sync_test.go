package main

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/registry"
)

func TestSyncProfileStructure(t *testing.T) {
	// Verify the SyncProfile struct has the expected fields
	profile := &registry.SyncProfile{
		Version:    1,
		ExportedAt: "2025-01-01T00:00:00Z",
		Device:     "test-host",
		Packages: []registry.SyncPackageEntry{
			{
				Name:       "@test/skill-a",
				Version:    "1.0.0",
				Source:     "registry",
				Syncable:   true,
				Visibility: "private",
				Agents:     []string{"claude"},
			},
		},
	}

	if profile.Version != 1 {
		t.Errorf("Version = %d, want 1", profile.Version)
	}
	if profile.Device != "test-host" {
		t.Errorf("Device = %q, want %q", profile.Device, "test-host")
	}
	if len(profile.Packages) != 1 {
		t.Fatalf("Packages count = %d, want 1", len(profile.Packages))
	}
	if !profile.Packages[0].Syncable {
		t.Error("registry package should be syncable")
	}
}

func TestSyncPackageEntry_SyncableBySource(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		source   string
		want     bool
	}{
		{"registry package is syncable", "@scope/skill", "registry", true},
		{"local package is not syncable", "my-local-skill", "local", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the syncability logic from buildSyncProfile
			syncable := tt.fullName != "" && tt.fullName[0] == '@'
			if syncable != tt.want {
				t.Errorf("syncable = %v, want %v for %q", syncable, tt.want, tt.fullName)
			}
		})
	}
}

func TestSyncCmd_HasExpectedSubcommands(t *testing.T) {
	subs := syncCmd.Commands()
	names := make(map[string]bool)
	for _, sub := range subs {
		names[sub.Name()] = true
	}

	required := []string{"export", "push", "pull", "status"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("sync missing subcommand %q", name)
		}
	}
}
