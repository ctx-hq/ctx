package app

import (
	"testing"
)

func TestAgentsTab_Loading(t *testing.T) {
	svc := &mockService{}
	tab := newAgentsTab(svc, 80, 24)

	if tab.loaded {
		t.Fatal("expected loaded=false initially")
	}

	view := tab.View()
	if view == "" {
		t.Fatal("expected loading view")
	}
}

func TestAgentsTab_Populated(t *testing.T) {
	svc := &mockService{}
	tab := newAgentsTab(svc, 80, 24)

	tab, _ = tab.Update(agentsDetectedMsg{
		Agents: []agentInfo{
			{Name: "claude", SkillsDir: "/home/.claude/skills", SkillCount: 3},
			{Name: "cursor", SkillsDir: "/home/.cursor/skills", SkillCount: 1},
		},
	})

	if !tab.loaded {
		t.Fatal("expected loaded=true")
	}
	if tab.empty {
		t.Fatal("expected empty=false")
	}
	if len(tab.table.Rows()) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(tab.table.Rows()))
	}
}

func TestAgentsTab_Empty(t *testing.T) {
	svc := &mockService{}
	tab := newAgentsTab(svc, 80, 24)

	tab, _ = tab.Update(agentsDetectedMsg{Agents: nil})

	if !tab.loaded {
		t.Fatal("expected loaded=true")
	}
	if !tab.empty {
		t.Fatal("expected empty=true")
	}
}
