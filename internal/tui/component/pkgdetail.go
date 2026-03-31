package component

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// PkgDetail key bindings.
var (
	pdKeyEsc = key.NewBinding(key.WithKeys("esc"))
)

// PkgDetailModel is a scrollable detail panel for displaying package information.
type PkgDetailModel struct {
	viewport viewport.Model
	content  string
	width    int
	height   int
	visible  bool
}

// NewPkgDetail creates a new package detail panel.
func NewPkgDetail() PkgDetailModel {
	vp := viewport.New()
	return PkgDetailModel{
		viewport: vp,
	}
}

// SetSize sets the width and height of the detail panel.
func (m *PkgDetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Account for border (1 char each side).
	innerW := w - 2
	innerH := h - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	m.viewport.SetWidth(innerW)
	m.viewport.SetHeight(innerH)
}

// SetContent formats and sets the title and body into the viewport.
func (m *PkgDetailModel) SetContent(title, body string) {
	styled := fmt.Sprintf("%s\n\n%s",
		tui.ListItemTitle.Render(title),
		body,
	)
	m.content = styled
	m.viewport.SetContent(styled)
}

// SetVisible shows or hides the detail panel.
func (m *PkgDetailModel) SetVisible(v bool) {
	m.visible = v
}

// Visible returns whether the detail panel is currently visible.
func (m PkgDetailModel) Visible() bool {
	return m.visible
}

// Init satisfies the BubbleTea model interface.
func (m PkgDetailModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the detail panel, including viewport scrolling
// and escape to hide.
func (m PkgDetailModel) Update(msg tea.Msg) (PkgDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, pdKeyEsc) {
			m.visible = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the detail panel. Returns empty string when not visible.
func (m PkgDetailModel) View() string {
	if !m.visible {
		return ""
	}

	border := tui.DetailBorder.
		Width(m.width).
		Height(m.height)

	content := m.viewport.View()

	return border.Render(lipgloss.PlaceHorizontal(
		m.width-2, lipgloss.Left, content,
	))
}
