package component

import (
	"testing"
)

func TestStatusBar_SetStatus(t *testing.T) {
	m := NewStatusBar()
	m.SetStatus("Installing...")
	if m.center != "Installing..." {
		t.Errorf("expected center %q, got %q", "Installing...", m.center)
	}
}

func TestStatusBar_SetWidth(t *testing.T) {
	m := NewStatusBar()
	m.SetWidth(120)
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
}

func TestStatusBar_ClearMsg(t *testing.T) {
	m := NewStatusBar()
	m.SetStatus("done")
	m, _ = m.Update(statusClearMsg{})
	if m.center != "" {
		t.Errorf("expected center to be cleared, got %q", m.center)
	}
}
