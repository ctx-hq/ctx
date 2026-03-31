package component

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTabs_NumberKeySwitch(t *testing.T) {
	m := NewTabs("All", "Skills", "MCP")

	// Press "1" → tab 0
	m, _ = m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if m.ActiveTab() != 0 {
		t.Errorf("expected active tab 0 after pressing 1, got %d", m.ActiveTab())
	}

	// Press "2" → tab 1
	m, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if m.ActiveTab() != 1 {
		t.Errorf("expected active tab 1 after pressing 2, got %d", m.ActiveTab())
	}

	// Press "3" → tab 2
	m, _ = m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	if m.ActiveTab() != 2 {
		t.Errorf("expected active tab 2 after pressing 3, got %d", m.ActiveTab())
	}
}

func TestTabs_WrapAround(t *testing.T) {
	m := NewTabs("A", "B", "C")
	m.SetActiveTab(2) // last tab

	// Tab forward from last → should wrap to first
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.ActiveTab() != 0 {
		t.Errorf("expected wrap to tab 0, got %d", m.ActiveTab())
	}

	// Shift+Tab backward from first → should wrap to last
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.ActiveTab() != 2 {
		t.Errorf("expected wrap to tab 2, got %d", m.ActiveTab())
	}
}

func TestTabs_SetWidth(t *testing.T) {
	m := NewTabs("A", "B")
	m.SetWidth(80)
	if m.width != 80 {
		t.Errorf("expected width 80, got %d", m.width)
	}
}
