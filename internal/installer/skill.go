package installer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/output"
)

// quietLinkKey suppresses "Linked to:" output when set in context.
type quietLinkKeyType struct{}

// QuietLinkKey can be set in context to suppress skill linking output.
// Use context.WithValue(ctx, installer.QuietLinkKey, true).
var QuietLinkKey = quietLinkKeyType{}

// LinkSkillToAgents links an installed skill to all detected agents and records
// the links in the LinkRegistry for later cleanup. If caller is non-empty, that
// agent is linked first and marked as the invoking agent. If targetAgents is
// non-nil, only those agents are linked instead of all detected agents.
// Returns the list of skill states for tracking in state.json.
func LinkSkillToAgents(ctx context.Context, installDir, skillName, fullName, caller string, targetAgents []agent.Agent) ([]installstate.SkillState, error) {
	quiet, _ := ctx.Value(QuietLinkKey).(bool)
	output.Verbose(ctx, "linking skill %s to detected agents", skillName)
	agents := targetAgents
	if agents == nil {
		agents = agent.DetectAll()
	}

	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}

	linked := make(map[string]bool)
	var linkedNames []string
	var skillStates []installstate.SkillState

	// Link caller agent first if specified
	if caller != "" {
		a, err := agent.FindByName(caller)
		if err != nil {
			output.Warn("Caller agent %q not recognized: %v", caller, err)
		} else {
			target := filepath.Join(a.SkillsDir(), skillName)
			if err := a.InstallSkill(installDir, skillName); err != nil {
				output.Warn("Failed to link to %s: %v", a.Name(), err)
				skillStates = append(skillStates, installstate.SkillState{Agent: a.Name(), SymlinkPath: target, Status: "broken"})
			} else {
				links.Add(fullName, LinkEntry{
					Agent:  a.Name(),
					Type:   LinkSymlink,
					Source: installDir,
					Target: target,
				})
				skillStates = append(skillStates, installstate.SkillState{Agent: a.Name(), SymlinkPath: target, Status: "ok"})
				linked[a.Name()] = true
				linkedNames = append(linkedNames, a.Name())
			}
		}
	}

	// Link remaining detected agents
	for _, a := range agents {
		if linked[a.Name()] {
			continue
		}
		target := filepath.Join(a.SkillsDir(), skillName)
		if err := a.InstallSkill(installDir, skillName); err != nil {
			output.Warn("Failed to link to %s: %v", a.Name(), err)
			skillStates = append(skillStates, installstate.SkillState{Agent: a.Name(), SymlinkPath: target, Status: "broken"})
			continue
		}

		links.Add(fullName, LinkEntry{
			Agent:  a.Name(),
			Type:   LinkSymlink,
			Source: installDir,
			Target: target,
		})
		skillStates = append(skillStates, installstate.SkillState{Agent: a.Name(), SymlinkPath: target, Status: "ok"})
		linked[a.Name()] = true
		linkedNames = append(linkedNames, a.Name())
	}

	if len(linked) == 0 {
		output.Warn("No agents detected. Use 'ctx link <agent>' to link manually.")
	} else if !quiet {
		output.PrintLinkedAgents(linkedNames)
	}

	_ = links.Save() // best effort
	return skillStates, nil
}

// UnlinkSkillFromAgents removes a skill from all detected agents.
func UnlinkSkillFromAgents(skillName string) error {
	agents := agent.DetectAll()
	for _, a := range agents {
		if err := a.RemoveSkill(skillName); err != nil {
			// Not an error if the skill wasn't linked
			continue
		}
	}
	return nil
}

// LinkSkillToAgent links a skill to a specific agent.
func LinkSkillToAgent(installDir, skillName, agentName string) error {
	a, err := agent.FindByName(agentName)
	if err != nil {
		return err
	}
	if err := a.InstallSkill(installDir, skillName); err != nil {
		return fmt.Errorf("link to %s: %w", agentName, err)
	}
	return nil
}
