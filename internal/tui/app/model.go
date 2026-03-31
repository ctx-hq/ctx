package app

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui/component"
)

// Root model key bindings.
var (
	keyQuit   = key.NewBinding(key.WithKeys("q"))
	keyCtrlC  = key.NewBinding(key.WithKeys("ctrl+c"))
	keyDoctor = key.NewBinding(key.WithKeys("d"))
)

// Model is the root TUI model that coordinates all components.
type Model struct {
	tabs      component.TabsModel
	installed installedTab
	discover  discoverTab
	agents    agentsTab
	statusBar component.StatusBarModel
	doctor    doctorOverlay
	detail    component.PkgDetailModel
	service   Service
	width     int
	height    int
	quitting  bool
}

// New creates a new root TUI model.
func New(svc Service) Model {
	tabs := component.NewTabs("Installed", "Discover", "Agents")
	sb := component.NewStatusBar()
	sb.SetLeft(" ctx")
	sb.SetRight("q:quit  d:doctor  ?:help ")

	return Model{
		tabs:      tabs,
		installed: newInstalledTab(svc, 80, 20),
		discover:  newDiscoverTab(svc, 80, 20),
		agents:    newAgentsTab(svc, 80, 20),
		statusBar: sb,
		doctor:    newDoctorOverlay(80, 20),
		detail:    component.NewPkgDetail(),
		service:   svc,
	}
}

// Init returns the initial commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.installed.Init(),
		m.agents.Init(),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSize()
		return m, nil

	case tea.KeyPressMsg:
		// Doctor overlay intercepts keys when visible
		if m.doctor.visible {
			var cmd tea.Cmd
			m.doctor, cmd = m.doctor.Update(msg)
			return m, cmd
		}

		// Detail overlay intercepts keys when visible
		if m.detail.Visible() {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}

		// Global keys
		if key.Matches(msg, keyQuit) || key.Matches(msg, keyCtrlC) {
			m.quitting = true
			return m, tea.Quit
		}
		if key.Matches(msg, keyDoctor) {
			cmd := m.doctor.Toggle(m.service)
			return m, cmd
		}

		// Tab switching
		oldTab := m.tabs.ActiveTab()
		m.tabs, _ = m.tabs.Update(msg)
		if m.tabs.ActiveTab() != oldTab {
			return m, nil
		}

		// Route to active tab
		return m, m.updateActiveTab(msg)

	case statusMsg:
		m.statusBar.SetStatus(msg.Text)
		return m, nil

	case installedLoadedMsg:
		var cmd tea.Cmd
		m.installed, cmd = m.installed.Update(msg)
		return m, cmd

	case searchResultMsg:
		var cmd tea.Cmd
		m.discover, cmd = m.discover.Update(msg)
		return m, cmd

	case agentsDetectedMsg:
		var cmd tea.Cmd
		m.agents, cmd = m.agents.Update(msg)
		return m, cmd

	case doctorResultMsg:
		var cmd tea.Cmd
		m.doctor, cmd = m.doctor.Update(msg)
		return m, cmd

	case switchTabMsg:
		m.tabs.SetActiveTab(msg.Tab)
		return m, nil
	}

	// Default: route to active tab
	return m, m.updateActiveTab(msg)
}

func (m *Model) updateActiveTab(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.tabs.ActiveTab() {
	case 0:
		m.installed, cmd = m.installed.Update(msg)
	case 1:
		m.discover, cmd = m.discover.Update(msg)
	case 2:
		m.agents, cmd = m.agents.Update(msg)
	}
	return cmd
}

// View renders the TUI.
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	tabBar := m.tabs.View()

	ch := m.contentHeight()
	var content string
	switch m.tabs.ActiveTab() {
	case 0:
		content = m.installed.View()
	case 1:
		content = m.discover.View()
	case 2:
		content = m.agents.View()
	}

	// Fit content to height
	contentBox := lipgloss.NewStyle().
		Width(m.width).
		Height(ch).
		Render(content)

	statusBar := m.statusBar.View()

	full := lipgloss.JoinVertical(lipgloss.Left, tabBar, contentBox, statusBar)

	// Overlay doctor if visible
	if m.doctor.visible {
		overlay := m.doctor.View()
		v := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay))
		v.AltScreen = true
		return v
	}

	// Overlay detail if visible
	if m.detail.Visible() {
		overlay := m.detail.View()
		v := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay))
		v.AltScreen = true
		return v
	}

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

// contentHeight returns the available height for the tab content.
func (m Model) contentHeight() int {
	// tab bar = 1 line, status bar = 1 line, borders = 2 lines
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) propagateSize() {
	m.tabs.SetWidth(m.width)
	m.statusBar.SetWidth(m.width)
	ch := m.contentHeight()
	m.installed.SetSize(m.width, ch)
	m.discover.SetSize(m.width, ch)
	m.agents.SetSize(m.width, ch)
	m.doctor.SetSize(m.width, m.height)
	m.detail.SetSize(m.width-4, m.height-4)
}
