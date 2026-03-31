package app

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/registry"
)

// mockService implements Service for testing.
type mockService struct {
	installed    []installer.InstalledPackage
	agents       []agentInfo
	checks       *doctor.Result
	packageState *installstate.PackageState
	files        []FileInfo
	fileContent  map[string]string
	skillContent map[string]string
	agentSkills  []AgentSkillEntry
	agentMCP     []AgentMCPEntry
	dirFiles     []FileInfo
	dirContent   map[string]string
}

func (s *mockService) ScanInstalled() ([]installer.InstalledPackage, error) {
	return s.installed, nil
}

func (s *mockService) Search(_ context.Context, _, _ string, _, _ int) (*registry.SearchResult, error) {
	return &registry.SearchResult{
		Packages: []registry.PackageInfo{
			{FullName: "@test/search-result", Type: "skill", Version: "1.0.0", Description: "A test result"},
		},
		Total: 1,
	}, nil
}

func (s *mockService) GetPackage(_ context.Context, _ string) (*registry.PackageDetail, error) {
	return nil, nil
}

func (s *mockService) Install(_ context.Context, _ string) (*installer.InstallResult, error) {
	return nil, nil
}

func (s *mockService) Remove(_ context.Context, _ string) error {
	return nil
}

func (s *mockService) DetectAgents() []agentInfo {
	return s.agents
}

func (s *mockService) RunDoctorChecks() *doctor.Result {
	return s.checks
}

func (s *mockService) GetPackageState(_ string) *installstate.PackageState {
	return s.packageState
}

func (s *mockService) ListPackageFiles(_ string) ([]FileInfo, error) {
	return s.files, nil
}

func (s *mockService) ReadPackageFile(_, fileName string) (string, error) {
	if s.fileContent != nil {
		if c, ok := s.fileContent[fileName]; ok {
			return c, nil
		}
	}
	return "", nil
}

func (s *mockService) GetSkillContent(fullName string) string {
	if s.skillContent != nil {
		return s.skillContent[fullName]
	}
	return ""
}

func (s *mockService) GetAgentDetail(_ string) ([]AgentSkillEntry, []AgentMCPEntry) {
	return s.agentSkills, s.agentMCP
}

func (s *mockService) ListDirFiles(_ string) ([]FileInfo, error) {
	return s.dirFiles, nil
}

func (s *mockService) ReadDirFile(_, name string) (string, error) {
	if s.dirContent != nil {
		if c, ok := s.dirContent[name]; ok {
			return c, nil
		}
	}
	return "", nil
}

func newTestModel() Model {
	// Use a past startTime so the 500ms grace period doesn't block test key events.
	svc := &mockService{
		installed: []installer.InstalledPackage{
			{FullName: "@hong/review", Version: "1.2.0", Type: "skill", Description: "Code review"},
			{FullName: "@mcp/github", Version: "0.5.0", Type: "mcp", Description: "GitHub MCP"},
		},
		agents: []agentInfo{
			{Name: "Claude Code", SkillsDir: "/home/.claude/skills", SkillCount: 3},
		},
		checks: &doctor.Result{
			Checks: []doctor.Check{
				{Name: "Version", Status: "pass", Detail: "v0.20.0"},
				{Name: "Auth", Status: "warn", Detail: "Not authenticated", Hint: "Run ctx auth login"},
			},
			PassCount: 1,
			WarnCount: 1,
		},
	}
	m := New(svc)
	m.startTime = time.Now().Add(-time.Second) // bypass startup grace period
	m.noDebounce = true                        // disable key debounce in tests
	// Simulate window size to make the model ready.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	return m
}

func TestModel_InitialMode(t *testing.T) {
	m := newTestModel()
	if m.mode != modeInstalled {
		t.Errorf("expected initial mode to be modeInstalled, got %d", m.mode)
	}
	if m.focus != focusList {
		t.Errorf("expected initial focus to be focusList, got %d", m.focus)
	}
	if !m.ready {
		t.Error("expected model to be ready after WindowSizeMsg")
	}
}

func TestModel_SwitchToSearch(t *testing.T) {
	m := newTestModel()
	// Press / to switch to search mode.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.mode != modeSearch {
		t.Errorf("expected mode to be modeSearch, got %d", m.mode)
	}
	if m.focus != focusSearch {
		t.Errorf("expected focus to be focusSearch, got %d", m.focus)
	}
}

func TestModel_SwitchToDoctor(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = updated.(Model)
	if m.mode != modeDoctor {
		t.Errorf("expected mode to be modeDoctor, got %d", m.mode)
	}
}

func TestModel_EscReturnsToInstalled(t *testing.T) {
	m := newTestModel()
	// Switch to doctor first.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = updated.(Model)
	if m.mode != modeDoctor {
		t.Fatalf("expected modeDoctor, got %d", m.mode)
	}
	// Press esc.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.mode != modeInstalled {
		t.Errorf("expected mode to return to modeInstalled, got %d", m.mode)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := newTestModel()
	if m.showHelp {
		t.Fatal("expected help to be hidden initially")
	}
	// Press ? to show help.
	updated, _ := m.Update(tea.KeyPressMsg{Code: '?'})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("expected help to be shown after pressing ?")
	}
	// Press ? again to hide.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = updated.(Model)
	if m.showHelp {
		t.Error("expected help to be hidden after pressing ? again")
	}
}

func TestModel_QuitSetsQuitting(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	m = updated.(Model)
	if !m.quitting {
		t.Error("expected quitting to be true after pressing q")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestModel_InstalledLoadedMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(installedLoadedMsg{
		Pkgs: []installer.InstalledPackage{
			{FullName: "@test/pkg", Version: "1.0.0", Type: "cli", Description: "Test package"},
		},
	})
	m = updated.(Model)
	if len(m.installed) != 1 {
		t.Errorf("expected 1 installed package, got %d", len(m.installed))
	}
	if m.installed[0].fullName != "@test/pkg" {
		t.Errorf("expected @test/pkg, got %s", m.installed[0].fullName)
	}
}

func TestModel_AgentsDetectedMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(agentsDetectedMsg{
		Agents: []agentInfo{
			{Name: "Cursor", SkillsDir: "/home/.cursor/skills", SkillCount: 2},
		},
	})
	m = updated.(Model)
	if len(m.agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(m.agents))
	}
}

func TestModel_DoctorResultMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(doctorResultMsg{
		Result: &doctor.Result{
			Checks: []doctor.Check{
				{Name: "Test", Status: "pass", Detail: "OK"},
			},
			PassCount: 1,
		},
	})
	m = updated.(Model)
	// Verify doctor list got the item (we can't easily inspect list items,
	// but the update should not panic).
	_ = updated
}

func TestModel_SearchEscReturnsFocus(t *testing.T) {
	m := newTestModel()
	// Enter search mode.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.focus != focusSearch {
		t.Fatalf("expected focusSearch, got %d", m.focus)
	}
	// Press esc to exit search input.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.focus != focusList {
		t.Errorf("expected focusList after esc in search, got %d", m.focus)
	}
}

func TestModel_ViewNotReady(t *testing.T) {
	svc := &mockService{}
	m := New(svc)
	m.startTime = time.Now().Add(-time.Second)
	m.noDebounce = true
	v := m.View()
	// Should show loading.
	if v.AltScreen != true {
		t.Error("expected AltScreen to be true")
	}
}

func TestModel_ViewReady(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if v.AltScreen != true {
		t.Error("expected AltScreen to be true")
	}
}

func newTestModelWithData() Model {
	m := newTestModel()
	// Simulate data loading (Init sends these async, we do it manually).
	updated, _ := m.Update(installedLoadedMsg{
		Pkgs: []installer.InstalledPackage{
			{FullName: "@hong/review", Version: "1.2.0", Type: "skill", Description: "Code review"},
			{FullName: "@mcp/github", Version: "0.5.0", Type: "mcp", Description: "GitHub MCP"},
			{FullName: "@ctx/ripgrep", Version: "14.1.0", Type: "cli", Description: "Fast search"},
		},
	})
	m = updated.(Model)
	updated, _ = m.Update(agentsDetectedMsg{
		Agents: []agentInfo{
			{Name: "Claude Code", SkillsDir: "/home/.claude/skills", SkillCount: 3},
		},
	})
	m = updated.(Model)
	updated, _ = m.Update(doctorResultMsg{
		Result: &doctor.Result{
			Checks:    []doctor.Check{{Name: "Version", Status: "pass", Detail: "v0.20.0"}},
			PassCount: 1,
		},
	})
	m = updated.(Model)
	return m
}

func TestModel_CursorNavigation(t *testing.T) {
	m := newTestModelWithData()

	// Should start at index 0.
	if m.pkgList.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.pkgList.Index())
	}

	// Press down arrow → index 1.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.pkgList.Index() != 1 {
		t.Errorf("after down: expected index 1, got %d", m.pkgList.Index())
	}

	// Press j → index 2.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)
	if m.pkgList.Index() != 2 {
		t.Errorf("after j: expected index 2, got %d", m.pkgList.Index())
	}

	// Press up arrow → index 1.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.pkgList.Index() != 1 {
		t.Errorf("after up: expected index 1, got %d", m.pkgList.Index())
	}

	// Press k → index 0.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)
	if m.pkgList.Index() != 0 {
		t.Errorf("after k: expected index 0, got %d", m.pkgList.Index())
	}
}

func TestModel_SwitchToAgents(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(Model)
	if m.mode != modeAgents {
		t.Errorf("expected modeAgents, got %d", m.mode)
	}
}

func TestModel_SwitchBackToPackages(t *testing.T) {
	m := newTestModel()
	// Go to agents.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(Model)
	// Go back to packages.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(Model)
	if m.mode != modeInstalled {
		t.Errorf("expected modeInstalled, got %d", m.mode)
	}
}

// ── New tests for agent filter, browse mode, and file content ──

func TestModel_TabSwitchesFocusToDetail(t *testing.T) {
	m := newTestModelWithData()
	if m.focus != focusList {
		t.Fatalf("expected focusList initially, got %d", m.focus)
	}

	// Tab → focus detail pane.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusDetail {
		t.Errorf("expected focusDetail after Tab, got %d", m.focus)
	}

	// Tab in detail → back to list (handled by detail focus handler).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusList {
		t.Errorf("expected focusList after Tab in detail, got %d", m.focus)
	}
}

func TestModel_EscFromDetailReturnsList(t *testing.T) {
	m := newTestModelWithData()
	// Tab → focus detail.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusDetail {
		t.Fatalf("expected focusDetail, got %d", m.focus)
	}
	// Esc → back to list.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.focus != focusList {
		t.Errorf("expected focusList after Esc, got %d", m.focus)
	}
}

func TestModel_BrowseModeEnter(t *testing.T) {
	svc := &mockService{
		installed: []installer.InstalledPackage{
			{FullName: "@hong/review", Version: "1.2.0", Type: "skill", Description: "Code review", InstallPath: "/tmp/test"},
		},
		files: []FileInfo{
			{Name: "SKILL.md", Size: 100},
			{Name: "prompt.txt", Size: 50},
		},
	}
	m := New(svc)
	m.startTime = time.Now().Add(-time.Second)
	m.noDebounce = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(installedLoadedMsg{Pkgs: svc.installed})
	m = updated.(Model)

	// Press Enter on the installed package should switch to modeBrowse.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeBrowse {
		t.Errorf("expected modeBrowse after Enter, got %d", m.mode)
	}
	if m.browsePackage != "@hong/review" {
		t.Errorf("expected browsePackage to be @hong/review, got %s", m.browsePackage)
	}
}

func TestModel_BrowseModeEsc(t *testing.T) {
	svc := &mockService{
		installed: []installer.InstalledPackage{
			{FullName: "@hong/review", Version: "1.2.0", Type: "skill", Description: "Code review", InstallPath: "/tmp/test"},
		},
		files: []FileInfo{
			{Name: "SKILL.md", Size: 100},
		},
	}
	m := New(svc)
	m.startTime = time.Now().Add(-time.Second)
	m.noDebounce = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(installedLoadedMsg{Pkgs: svc.installed})
	m = updated.(Model)

	// Enter browse mode.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("expected modeBrowse, got %d", m.mode)
	}

	// Press Esc to return to installed.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.mode != modeInstalled {
		t.Errorf("expected modeInstalled after Esc, got %d", m.mode)
	}
}

func TestModel_FilesLoadedMsg(t *testing.T) {
	m := newTestModel()
	m.mode = modeBrowse
	m.browsePackage = "@hong/review"

	updated, _ := m.Update(filesLoadedMsg{
		Files: []FileInfo{
			{Name: "SKILL.md", Size: 100},
			{Name: "prompt.txt", Size: 50},
		},
	})
	m = updated.(Model)
	if len(m.fileList.Items()) != 2 {
		t.Errorf("expected 2 file items, got %d", len(m.fileList.Items()))
	}
}

func TestModel_FileContentMsg(t *testing.T) {
	m := newTestModel()
	m.mode = modeBrowse
	m.browsePackage = "@hong/review"

	// Should not panic on file content message.
	updated, _ := m.Update(fileContentMsg{
		Name:    "test.go",
		Content: "package main\n\nfunc main() {}\n",
	})
	_ = updated.(Model)
}

func TestModel_AgentEnterBrowsesSkillsDir(t *testing.T) {
	svc := &mockService{
		installed: []installer.InstalledPackage{},
		agents: []agentInfo{
			{Name: "Claude Code", SkillsDir: "/home/.claude/skills", SkillCount: 2},
		},
		dirFiles: []FileInfo{
			{Name: "review", Size: 0, IsDir: true},
			{Name: "gc", Size: 0, IsDir: true},
		},
	}
	m := New(svc)
	m.startTime = time.Now().Add(-time.Second)
	m.noDebounce = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(agentsDetectedMsg{Agents: svc.agents})
	m = updated.(Model)

	// Switch to agents mode.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(Model)
	if m.mode != modeAgents {
		t.Fatalf("expected modeAgents, got %d", m.mode)
	}

	// Press Enter on agent.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeBrowse {
		t.Errorf("expected modeBrowse after Enter on agent, got %d", m.mode)
	}
	if m.browseDir != "/home/.claude/skills" {
		t.Errorf("expected browseDir to be skills dir, got %s", m.browseDir)
	}

	// Esc should return to agents, not installed.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.mode != modeAgents {
		t.Errorf("expected modeAgents after Esc from agent browse, got %d", m.mode)
	}
}

func TestModel_TabDoesNotCycleInSearchMode(t *testing.T) {
	m := newTestModelWithData()

	// Enter search mode.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.mode != modeSearch {
		t.Fatalf("expected modeSearch, got %d", m.mode)
	}

	// Tab in search mode should NOT change agentFilter.
	beforeFilter := m.agentFilter
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.agentFilter != beforeFilter {
		t.Errorf("expected agentFilter unchanged in search mode, was %d now %d", beforeFilter, m.agentFilter)
	}
}
