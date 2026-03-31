package component

import (
	"testing"
)

func TestPkgDetail_SetContent(t *testing.T) {
	m := NewPkgDetail()
	m.SetSize(60, 20)
	m.SetContent("my-package", "A great package for doing things.")

	if m.content == "" {
		t.Error("expected content to be populated after SetContent")
	}
}

func TestPkgDetail_SetVisible(t *testing.T) {
	m := NewPkgDetail()

	if m.Visible() {
		t.Error("expected panel to be hidden by default")
	}

	m.SetVisible(true)
	if !m.Visible() {
		t.Error("expected panel to be visible after SetVisible(true)")
	}

	m.SetVisible(false)
	if m.Visible() {
		t.Error("expected panel to be hidden after SetVisible(false)")
	}
}

func TestPkgDetail_ViewEmptyWhenNotVisible(t *testing.T) {
	m := NewPkgDetail()
	m.SetSize(60, 20)
	m.SetContent("title", "body text here")
	m.SetVisible(false)

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when not visible, got %q", view)
	}
}

func TestPkgDetail_ViewNonEmptyWhenVisible(t *testing.T) {
	m := NewPkgDetail()
	m.SetSize(60, 20)
	m.SetContent("title", "body text here")
	m.SetVisible(true)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view when visible")
	}
}
