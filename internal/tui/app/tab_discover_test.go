package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/registry"
)

func TestDiscoverTab_SearchOnEnter(t *testing.T) {
	svc := &mockService{
		searchResult: &registry.SearchResult{
			Packages: []registry.PackageInfo{
				{FullName: "@test/skill", Type: "skill", Description: "A test skill"},
			},
			Total: 1,
		},
	}
	tab := newDiscoverTab(svc, 80, 24)

	// Set a value in the input
	tab.searchInput.SetValue("test")

	// Simulate enter key
	tab, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !tab.searching {
		t.Fatal("expected searching=true after enter")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestDiscoverTab_ResultsPopulated(t *testing.T) {
	svc := &mockService{}
	tab := newDiscoverTab(svc, 80, 24)

	tab, _ = tab.Update(searchResultMsg{
		Result: &registry.SearchResult{
			Packages: []registry.PackageInfo{
				{FullName: "@test/a", Type: "skill"},
				{FullName: "@test/b", Type: "mcp"},
			},
			Total: 2,
		},
	})

	if tab.searching {
		t.Fatal("expected searching=false after results")
	}
	if len(tab.results.Items()) != 2 {
		t.Fatalf("expected 2 results, got %d", len(tab.results.Items()))
	}
}
