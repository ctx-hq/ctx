package inline

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/tui/component"
)

// mockAgent implements agent.Agent for testing.
type mockAgent struct {
	name string
}

func (a *mockAgent) Name() string                                     { return a.name }
func (a *mockAgent) Detected() bool                                   { return true }
func (a *mockAgent) SkillsDir() string                                { return "/tmp/" + a.name }
func (a *mockAgent) InstallSkill(_, _ string) error                   { return nil }
func (a *mockAgent) RemoveSkill(_ string) error                       { return nil }
func (a *mockAgent) AddMCP(_ string, _ agent.MCPConfig) error         { return nil }
func (a *mockAgent) RemoveMCP(_ string) error                         { return nil }

// Verify mockAgent satisfies agent.Agent.
var _ agent.Agent = (*mockAgent)(nil)

func TestAgentSelectModel_PreSelection(t *testing.T) {
	agents := []agent.Agent{
		&mockAgent{name: "claude"},
		&mockAgent{name: "cursor"},
		&mockAgent{name: "windsurf"},
	}

	// Build items the same way agentSelectModel does
	items := make([]component.MultiSelectorItem, len(agents))
	for i, a := range agents {
		items[i] = component.MultiSelectorItem{
			Label:    a.Name(),
			Selected: true,
		}
	}
	ms := component.NewMultiSelector(items)
	indices := ms.SelectedIndices()

	if len(indices) != 3 {
		t.Fatalf("expected 3 pre-selected, got %d", len(indices))
	}
	for i, idx := range indices {
		if idx != i {
			t.Fatalf("expected index %d, got %d", i, idx)
		}
	}
}

func TestAgentSelectModel_SelectedAgents(t *testing.T) {
	agents := []agent.Agent{
		&mockAgent{name: "claude"},
		&mockAgent{name: "cursor"},
	}

	model := newAgentSelectModel(agents)
	selected := model.selectedAgents()
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected agents, got %d", len(selected))
	}
	if selected[0].Name() != "claude" {
		t.Fatalf("expected first agent 'claude', got %q", selected[0].Name())
	}
	if selected[1].Name() != "cursor" {
		t.Fatalf("expected second agent 'cursor', got %q", selected[1].Name())
	}
}
