package inline

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/ctx-hq/ctx/internal/tui/component"
)

// itemSelectModel wraps the multi-selector for generic item selection.
type itemSelectModel struct {
	label     string
	selector  component.MultiSelectorModel
	confirmed bool
	cancelled bool
}

func newItemSelectModel(items []component.MultiSelectorItem, label string) itemSelectModel {
	return itemSelectModel{
		label:    label,
		selector: component.NewMultiSelector(items),
	}
}

func (m itemSelectModel) Init() tea.Cmd { return nil }

func (m itemSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case component.ConfirmMsg:
		m.confirmed = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m itemSelectModel) View() tea.View {
	if m.confirmed || m.cancelled {
		return tea.NewView("")
	}
	return tea.NewView("  " + m.label + "\n" + m.selector.View())
}

// SelectFromItems shows an inline multi-selector for choosing from a list of items.
// Returns the indices of selected items. Non-TTY: returns all indices.
func SelectFromItems(items []component.MultiSelectorItem, label string) ([]int, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Non-TTY: return all indices
		indices := make([]int, len(items))
		for i := range items {
			indices[i] = i
		}
		return indices, nil
	}

	model := newItemSelectModel(items, label)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	m := final.(itemSelectModel)
	if m.cancelled {
		return nil, nil
	}
	return m.selector.SelectedIndices(), nil
}
