package inline

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/tui/component"
)

// agentSelectModel wraps the multi-selector component for agent selection.
type agentSelectModel struct {
	selector  component.MultiSelectorModel
	agents    []agent.Agent
	confirmed bool
	cancelled bool
}

func newAgentSelectModel(agents []agent.Agent) agentSelectModel {
	items := make([]component.MultiSelectorItem, len(agents))
	for i, a := range agents {
		items[i] = component.MultiSelectorItem{
			Label:    a.Name(),
			Selected: true, // pre-select all
		}
	}
	return agentSelectModel{
		selector: component.NewMultiSelector(items),
		agents:   agents,
	}
}

func (m agentSelectModel) Init() tea.Cmd { return nil }

func (m agentSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m agentSelectModel) View() tea.View {
	if m.confirmed || m.cancelled {
		return tea.NewView("")
	}
	return tea.NewView("  Select target agents:\n" + m.selector.View())
}

// selectedAgents returns the agents corresponding to selected indices.
func (m agentSelectModel) selectedAgents() []agent.Agent {
	indices := m.selector.SelectedIndices()
	result := make([]agent.Agent, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(m.agents) {
			result = append(result, m.agents[idx])
		}
	}
	return result
}

// SelectAgents shows an inline multi-selector for choosing target agents.
// Pre-selects all agents. Non-TTY: returns all agents unchanged.
func SelectAgents(agents []agent.Agent) ([]agent.Agent, error) {
	if len(agents) == 0 {
		return nil, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return agents, nil
	}

	model := newAgentSelectModel(agents)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	m := final.(agentSelectModel)
	if m.cancelled {
		return nil, nil
	}
	return m.selectedAgents(), nil
}
