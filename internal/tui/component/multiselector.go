package component

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// Multi-selector key bindings.
var (
	msKeyDown    = key.NewBinding(key.WithKeys("j", "down"))
	msKeyUp      = key.NewBinding(key.WithKeys("k", "up"))
	msKeyToggle  = key.NewBinding(key.WithKeys("space", "x"))
	msKeyAll     = key.NewBinding(key.WithKeys("a"))
	msKeyNone    = key.NewBinding(key.WithKeys("n"))
	msKeyConfirm = key.NewBinding(key.WithKeys("enter"))
)

// ConfirmMsg is emitted when the user confirms the selection.
type ConfirmMsg struct {
	Indices []int
}

// MultiSelectorItem represents a selectable item in the multi-selector.
type MultiSelectorItem struct {
	Label    string
	Selected bool
}

// MultiSelectorModel is a multi-select checkbox list component.
type MultiSelectorModel struct {
	items  []MultiSelectorItem
	cursor int
	width  int
	height int
}

// NewMultiSelector creates a new multi-selector with the given items.
func NewMultiSelector(items []MultiSelectorItem) MultiSelectorModel {
	return MultiSelectorModel{
		items: items,
	}
}

// Init satisfies the BubbleTea model interface.
func (m MultiSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles key messages for the multi-selector.
func (m MultiSelectorModel) Update(msg tea.Msg) (MultiSelectorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		n := len(m.items)
		if n == 0 {
			return m, nil
		}
		switch {
		case key.Matches(msg, msKeyDown):
			m.cursor = (m.cursor + 1) % n
		case key.Matches(msg, msKeyUp):
			m.cursor = (m.cursor - 1 + n) % n
		case key.Matches(msg, msKeyToggle):
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		case key.Matches(msg, msKeyAll):
			for i := range m.items {
				m.items[i].Selected = true
			}
		case key.Matches(msg, msKeyNone):
			for i := range m.items {
				m.items[i].Selected = false
			}
		case key.Matches(msg, msKeyConfirm):
			return m, func() tea.Msg {
				return ConfirmMsg{Indices: m.SelectedIndices()}
			}
		}
	}
	return m, nil
}

// View renders the multi-selector as a checkbox list.
func (m MultiSelectorModel) View() string {
	if len(m.items) == 0 {
		return ""
	}

	var b strings.Builder
	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		checkbox := "\u2610" // ☐
		if item.Selected {
			checkbox = "\u2611" // ☑
		}

		label := item.Label
		if i == m.cursor {
			label = tui.ListItemTitle.Render(label)
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checkbox, label))
	}
	return b.String()
}

// SelectedIndices returns the indices of all selected items.
func (m MultiSelectorModel) SelectedIndices() []int {
	var indices []int
	for i, item := range m.items {
		if item.Selected {
			indices = append(indices, i)
		}
	}
	return indices
}
