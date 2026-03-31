package app

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/installer"
)

func TestInstalledTab_LoadTransition(t *testing.T) {
	svc := &mockService{}
	tab := newInstalledTab(svc, 80, 24)

	if tab.loaded {
		t.Fatal("expected loaded=false initially")
	}

	// Simulate loaded message
	tab, _ = tab.Update(installedLoadedMsg{
		Pkgs: []installer.InstalledPackage{
			{FullName: "@test/pkg", Version: "1.0.0", Type: "skill"},
		},
	})

	if !tab.loaded {
		t.Fatal("expected loaded=true after message")
	}
	if len(tab.list.Items()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(tab.list.Items()))
	}
}

func TestInstalledTab_EmptyList(t *testing.T) {
	svc := &mockService{}
	tab := newInstalledTab(svc, 80, 24)

	tab, _ = tab.Update(installedLoadedMsg{Pkgs: nil})

	if !tab.loaded {
		t.Fatal("expected loaded=true")
	}

	view := tab.View()
	if view == "" {
		t.Fatal("expected non-empty view for empty state")
	}
}
