package app

import (
	"fmt"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

// agentsTab displays detected AI agents.
type agentsTab struct {
	table   table.Model
	service Service
	loaded  bool
	empty   bool
}

func newAgentsTab(svc Service, width, height int) agentsTab {
	cols := []table.Column{
		{Title: "Agent", Width: 20},
		{Title: "Skills Dir", Width: 40},
		{Title: "Skills", Width: 8},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithHeight(height-2),
		table.WithWidth(width),
	)
	t.Focus()

	return agentsTab{
		table:   t,
		service: svc,
	}
}

func (t agentsTab) Init() tea.Cmd {
	svc := t.service
	return func() tea.Msg {
		agents := svc.DetectAgents()
		return agentsDetectedMsg{Agents: agents}
	}
}

func (t agentsTab) Update(msg tea.Msg) (agentsTab, tea.Cmd) {
	switch msg := msg.(type) {
	case agentsDetectedMsg:
		t.loaded = true
		if len(msg.Agents) == 0 {
			t.empty = true
			return t, nil
		}
		rows := make([]table.Row, len(msg.Agents))
		for i, a := range msg.Agents {
			rows[i] = table.Row{a.Name, a.SkillsDir, fmt.Sprintf("%d", a.SkillCount)}
		}
		t.table.SetRows(rows)
		return t, nil
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

func (t agentsTab) View() string {
	if !t.loaded {
		return "  Detecting agents..."
	}
	if t.empty {
		return "  No agents detected.\n\n  Install Claude Code, Cursor, or Windsurf to get started."
	}
	return t.table.View()
}

// SetSize updates the table dimensions.
func (t *agentsTab) SetSize(w, h int) {
	t.table.SetHeight(h - 2)
	skillsW := 8
	agentW := w / 4
	dirW := w - agentW - skillsW
	t.table.SetColumns([]table.Column{
		{Title: "Agent", Width: agentW},
		{Title: "Skills Dir", Width: dirW},
		{Title: "Skills", Width: skillsW},
	})
}
