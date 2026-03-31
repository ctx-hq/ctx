package component

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newTestItems() []MultiSelectorItem {
	return []MultiSelectorItem{
		{Label: "Alpha"},
		{Label: "Beta"},
		{Label: "Gamma"},
	}
}

func TestMultiSelector_ToggleIndividual(t *testing.T) {
	m := NewMultiSelector(newTestItems())

	// Toggle first item with space
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !m.items[0].Selected {
		t.Error("expected item 0 to be selected after toggle")
	}

	// Toggle again to deselect
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.items[0].Selected {
		t.Error("expected item 0 to be deselected after second toggle")
	}
}

func TestMultiSelector_SelectAll(t *testing.T) {
	m := NewMultiSelector(newTestItems())

	// Select all
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	for i, item := range m.items {
		if !item.Selected {
			t.Errorf("expected item %d to be selected after select all", i)
		}
	}
}

func TestMultiSelector_DeselectAll(t *testing.T) {
	m := NewMultiSelector(newTestItems())

	// Select all first
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	// Deselect all
	m, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	for i, item := range m.items {
		if item.Selected {
			t.Errorf("expected item %d to be deselected after deselect all", i)
		}
	}
}

func TestMultiSelector_CursorWrap(t *testing.T) {
	m := NewMultiSelector(newTestItems())

	// Cursor starts at 0; move up should wrap to last
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 2 {
		t.Errorf("expected cursor to wrap to 2, got %d", m.cursor)
	}

	// Move down from last should wrap to first
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", m.cursor)
	}
}

func TestMultiSelector_ConfirmReturnsCorrectIndices(t *testing.T) {
	m := NewMultiSelector(newTestItems())

	// Select items 0 and 2
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})  // toggle 0
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})    // move to 1
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})    // move to 2
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})  // toggle 2

	indices := m.SelectedIndices()
	expected := []int{0, 2}
	if !reflect.DeepEqual(indices, expected) {
		t.Errorf("expected selected indices %v, got %v", expected, indices)
	}

	// Confirm emits a command
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected confirm command, got nil")
	}
	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if !reflect.DeepEqual(confirmMsg.Indices, expected) {
		t.Errorf("expected ConfirmMsg indices %v, got %v", expected, confirmMsg.Indices)
	}
}
