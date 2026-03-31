// Package component provides reusable BubbleTea UI components for the ctx TUI.
package component

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// Tab key bindings.
var (
	tabKeyOne = key.NewBinding(key.WithKeys("1"))
	tabKeyTwo = key.NewBinding(key.WithKeys("2"))
	tabKeyThr = key.NewBinding(key.WithKeys("3"))
	tabKeyTab = key.NewBinding(key.WithKeys("tab"))
	tabKeyBtb = key.NewBinding(key.WithKeys("shift+tab"))
)

// TabsModel is a horizontal tab bar component.
type TabsModel struct {
	titles    []string
	activeTab int
	width     int
}

// NewTabs creates a new tab bar with the given tab titles.
func NewTabs(titles ...string) TabsModel {
	return TabsModel{
		titles: titles,
	}
}

// SetWidth sets the available width for the tab bar.
func (m *TabsModel) SetWidth(w int) {
	m.width = w
}

// ActiveTab returns the index of the currently active tab.
func (m TabsModel) ActiveTab() int {
	return m.activeTab
}

// SetActiveTab sets the active tab to the given index. It clamps to valid range.
func (m *TabsModel) SetActiveTab(i int) {
	if len(m.titles) == 0 {
		return
	}
	if i < 0 {
		i = 0
	} else if i >= len(m.titles) {
		i = len(m.titles) - 1
	}
	m.activeTab = i
}

// Init satisfies the BubbleTea model interface.
func (m TabsModel) Init() tea.Cmd {
	return nil
}

// Update handles key messages for tab switching.
func (m TabsModel) Update(msg tea.Msg) (TabsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		n := len(m.titles)
		if n == 0 {
			return m, nil
		}
		switch {
		case key.Matches(msg, tabKeyOne):
			if n >= 1 {
				m.activeTab = 0
			}
		case key.Matches(msg, tabKeyTwo):
			if n >= 2 {
				m.activeTab = 1
			}
		case key.Matches(msg, tabKeyThr):
			if n >= 3 {
				m.activeTab = 2
			}
		case key.Matches(msg, tabKeyTab):
			m.activeTab = (m.activeTab + 1) % n
		case key.Matches(msg, tabKeyBtb):
			m.activeTab = (m.activeTab - 1 + n) % n
		}
	}
	return m, nil
}

// View renders the tab bar as a horizontal row.
func (m TabsModel) View() string {
	if len(m.titles) == 0 {
		return ""
	}

	var tabs []string
	for i, title := range m.titles {
		if i == m.activeTab {
			tabs = append(tabs, tui.ActiveTab.Render("> "+title))
		} else {
			tabs = append(tabs, tui.InactiveTab.Render("  "+title))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	bar := tui.TabBar.Width(m.width).Render(row)
	return bar
}

