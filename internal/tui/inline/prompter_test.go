package inline

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// charMsg creates a KeyPressMsg for a printable character.
func charMsg(c rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: c, Text: string(c)}
}

// enterMsg creates a KeyPressMsg for the Enter key.
func enterMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// downMsg creates a KeyPressMsg for the Down arrow key.
func downMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyDown}
}

// upMsg creates a KeyPressMsg for the Up arrow key.
func upMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyUp}
}

func TestTextModel_SubmitWithValue(t *testing.T) {
	m := newTextModel("Name", "default-name")
	// Simulate typing "hello"
	var model tea.Model = m
	model, _ = model.Update(charMsg('h'))
	model, _ = model.Update(charMsg('e'))
	model, _ = model.Update(charMsg('l'))
	model, _ = model.Update(charMsg('l'))
	model, _ = model.Update(charMsg('o'))

	// Submit
	model, _ = model.Update(enterMsg())
	tm := model.(textModel)
	if !tm.submitted {
		t.Fatal("expected submitted to be true")
	}
	got := tm.result()
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestTextModel_EmptyReturnsDefault(t *testing.T) {
	m := newTextModel("Name", "fallback")
	var model tea.Model = m
	// Submit immediately with no input
	model, _ = model.Update(enterMsg())
	tm := model.(textModel)
	if !tm.submitted {
		t.Fatal("expected submitted to be true")
	}
	got := tm.result()
	if got != "fallback" {
		t.Fatalf("expected 'fallback', got %q", got)
	}
}

func TestConfirmModel_YesInput(t *testing.T) {
	m := newConfirmModel("Continue?", false)
	var model tea.Model = m
	model, _ = model.Update(charMsg('y'))
	model, _ = model.Update(enterMsg())
	cm := model.(confirmModel)
	if !cm.submitted {
		t.Fatal("expected submitted")
	}
	if !cm.result() {
		t.Fatal("expected true for 'y' input")
	}
}

func TestConfirmModel_NoInput(t *testing.T) {
	m := newConfirmModel("Continue?", true)
	var model tea.Model = m
	model, _ = model.Update(charMsg('n'))
	model, _ = model.Update(enterMsg())
	cm := model.(confirmModel)
	if cm.result() {
		t.Fatal("expected false for 'n' input")
	}
}

func TestConfirmModel_EmptyReturnsDefault(t *testing.T) {
	m := newConfirmModel("Continue?", true)
	var model tea.Model = m
	model, _ = model.Update(enterMsg())
	cm := model.(confirmModel)
	if !cm.result() {
		t.Fatal("expected default true on empty input")
	}
}

func TestConfirmModel_EmptyReturnsFalseDefault(t *testing.T) {
	m := newConfirmModel("Continue?", false)
	var model tea.Model = m
	model, _ = model.Update(enterMsg())
	cm := model.(confirmModel)
	if cm.result() {
		t.Fatal("expected default false on empty input")
	}
}

func TestSelectModel_CursorMovement(t *testing.T) {
	m := newSelectModel("Pick", []string{"a", "b", "c"}, 0)
	var model tea.Model = m

	// Move down
	model, _ = model.Update(downMsg())
	sm := model.(selectModel)
	if sm.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", sm.cursor)
	}

	// Move down again
	model, _ = model.Update(downMsg())
	sm = model.(selectModel)
	if sm.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", sm.cursor)
	}

	// Wrap around
	model, _ = model.Update(downMsg())
	sm = model.(selectModel)
	if sm.cursor != 0 {
		t.Fatalf("expected cursor 0 (wrap), got %d", sm.cursor)
	}
}

func TestSelectModel_CursorUp(t *testing.T) {
	m := newSelectModel("Pick", []string{"a", "b", "c"}, 0)
	var model tea.Model = m

	// Move up from 0 wraps to end
	model, _ = model.Update(upMsg())
	sm := model.(selectModel)
	if sm.cursor != 2 {
		t.Fatalf("expected cursor 2 (wrap), got %d", sm.cursor)
	}
}

func TestSelectModel_Selection(t *testing.T) {
	m := newSelectModel("Pick", []string{"a", "b", "c"}, 1)
	var model tea.Model = m

	// Move down from default (1) to 2, then submit
	model, _ = model.Update(downMsg())
	model, _ = model.Update(enterMsg())
	sm := model.(selectModel)
	if !sm.submitted {
		t.Fatal("expected submitted")
	}
	if sm.cursor != 2 {
		t.Fatalf("expected selected index 2, got %d", sm.cursor)
	}
}

func TestSelectModel_DefaultIdx(t *testing.T) {
	m := newSelectModel("Pick", []string{"a", "b", "c"}, 2)
	if m.cursor != 2 {
		t.Fatalf("expected default cursor 2, got %d", m.cursor)
	}
}
