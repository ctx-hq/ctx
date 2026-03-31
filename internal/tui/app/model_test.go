package app

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/registry"
)

// mockService implements Service for testing.
type mockService struct {
	installed    []installer.InstalledPackage
	searchResult *registry.SearchResult
	agents       []agentInfo
}

func (m *mockService) ScanInstalled() ([]installer.InstalledPackage, error) {
	return m.installed, nil
}

func (m *mockService) Search(_ context.Context, _, _ string, _, _ int) (*registry.SearchResult, error) {
	if m.searchResult != nil {
		return m.searchResult, nil
	}
	return &registry.SearchResult{}, nil
}

func (m *mockService) GetPackage(_ context.Context, _ string) (*registry.PackageDetail, error) {
	return &registry.PackageDetail{}, nil
}

func (m *mockService) Install(_ context.Context, _ string) (*installer.InstallResult, error) {
	return &installer.InstallResult{}, nil
}

func (m *mockService) Remove(_ context.Context, _ string) error {
	return nil
}

func (m *mockService) DetectAgents() []agentInfo {
	return m.agents
}

func (m *mockService) RunDoctorChecks() *doctor.Result {
	return &doctor.Result{
		Checks:    []doctor.Check{{Name: "test", Status: "pass", Detail: "ok"}},
		PassCount: 1,
	}
}

func TestModel_TabSwitching(t *testing.T) {
	svc := &mockService{}
	m := New(svc)

	if m.tabs.ActiveTab() != 0 {
		t.Fatal("expected initial tab 0")
	}

	// Switch to tab 2 via switchTabMsg
	result, _ := m.Update(switchTabMsg{Tab: 2})
	m = result.(Model)

	if m.tabs.ActiveTab() != 2 {
		t.Fatalf("expected tab 2, got %d", m.tabs.ActiveTab())
	}
}

func TestModel_Quit(t *testing.T) {
	svc := &mockService{}
	m := New(svc)

	result, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = result.(Model)

	if !m.quitting {
		t.Fatal("expected quitting=true")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModel_Resize(t *testing.T) {
	svc := &mockService{}
	m := New(svc)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)

	if m.width != 120 {
		t.Fatalf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Fatalf("expected height=40, got %d", m.height)
	}
}
