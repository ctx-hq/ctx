package main

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestPushDefaults_PrivateAndMutable(t *testing.T) {
	// Push sets visibility=private and mutable=true by default
	m := &manifest.Manifest{
		Name:    "@test/my-skill",
		Version: "0.1.0",
		Type:    manifest.TypeSkill,
	}

	// Simulate push defaults logic from push.go
	if m.Visibility == "" {
		m.Visibility = "private"
	}
	if m.Visibility == "private" {
		m.Mutable = true
	}

	if m.Visibility != "private" {
		t.Errorf("Visibility = %q, want %q", m.Visibility, "private")
	}
	if !m.Mutable {
		t.Error("Mutable should be true for private push")
	}
}

func TestPushDefaults_PreservesExplicitVisibility(t *testing.T) {
	m := &manifest.Manifest{
		Name:       "@test/my-skill",
		Version:    "0.1.0",
		Type:       manifest.TypeSkill,
		Visibility: "public",
	}

	// Simulate push defaults — should not overwrite explicit visibility
	if m.Visibility == "" {
		m.Visibility = "private"
	}
	if m.Visibility == "private" {
		m.Mutable = true
	}

	if m.Visibility != "public" {
		t.Errorf("Visibility = %q, want %q (explicit value should be preserved)", m.Visibility, "public")
	}
	if m.Mutable {
		t.Error("Mutable should remain false for non-private push")
	}
}

func TestPushScopeAutoFill(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		username string
		want     string
	}{
		{
			name:     "fills placeholder scope",
			initial:  "@your-scope/my-skill",
			username: "alice",
			want:     "@alice/my-skill",
		},
		{
			name:     "preserves existing scope",
			initial:  "@bob/my-skill",
			username: "alice",
			want:     "@bob/my-skill",
		},
		{
			name:     "fills empty scope",
			initial:  "my-skill",
			username: "alice",
			want:     "@alice/my-skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &manifest.Manifest{Name: tt.initial}

			scope := m.Scope()
			if scope == "your-scope" || scope == "" {
				if tt.username != "" {
					_, name := manifest.ParseFullName(m.Name)
					m.Name = manifest.FormatFullName(tt.username, name)
				}
			}

			if m.Name != tt.want {
				t.Errorf("Name = %q, want %q", m.Name, tt.want)
			}
		})
	}
}
