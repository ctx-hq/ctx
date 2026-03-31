package component

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// statusClearMsg clears the status bar center text.
type statusClearMsg struct{}

// StatusBarModel is a full-width status bar with left, center, and right sections.
type StatusBarModel struct {
	left   string
	center string
	right  string
	width  int
}

// NewStatusBar creates a new empty status bar.
func NewStatusBar() StatusBarModel {
	return StatusBarModel{}
}

// SetLeft sets the left section text.
func (m *StatusBarModel) SetLeft(s string) {
	m.left = s
}

// SetCenter sets the center section text.
func (m *StatusBarModel) SetCenter(s string) {
	m.center = s
}

// SetRight sets the right section text.
func (m *StatusBarModel) SetRight(s string) {
	m.right = s
}

// SetWidth sets the available width for the status bar.
func (m *StatusBarModel) SetWidth(w int) {
	m.width = w
}

// SetStatus sets the center text as a status message.
func (m *StatusBarModel) SetStatus(msg string) {
	m.center = msg
}

// Init satisfies the BubbleTea model interface.
func (m StatusBarModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the status bar.
func (m StatusBarModel) Update(msg tea.Msg) (StatusBarModel, tea.Cmd) {
	switch msg.(type) {
	case statusClearMsg:
		m.center = ""
	}
	return m, nil
}

// View renders the status bar as a full-width string.
func (m StatusBarModel) View() string {
	w := m.width
	if w <= 0 {
		return ""
	}

	leftRendered := tui.StatusBarLeft.Render(m.left)
	rightRendered := tui.StatusBarRight.Render(m.right)

	leftWidth := lipgloss.Width(leftRendered)
	rightWidth := lipgloss.Width(rightRendered)

	// Calculate center padding so the center text is roughly centered,
	// and the right text is right-aligned.
	centerAvail := w - leftWidth - rightWidth
	if centerAvail < 0 {
		centerAvail = 0
	}

	centerText := m.center
	centerWidth := lipgloss.Width(centerText)

	var gap int
	if centerAvail > centerWidth {
		gap = (centerAvail - centerWidth) / 2
	}

	var b strings.Builder
	b.WriteString(leftRendered)
	if gap > 0 {
		b.WriteString(strings.Repeat(" ", gap))
	}
	b.WriteString(centerText)

	remaining := w - leftWidth - gap - centerWidth - rightWidth
	if remaining > 0 {
		b.WriteString(strings.Repeat(" ", remaining))
	}
	b.WriteString(rightRendered)

	return tui.StatusBar.Width(w).Render(b.String())
}
