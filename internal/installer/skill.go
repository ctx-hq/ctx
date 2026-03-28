package installer

import (
	"fmt"
	"path/filepath"

	"github.com/getctx/ctx/internal/agent"
	"github.com/getctx/ctx/internal/output"
)

// LinkSkillToAgents links an installed skill to all detected agents and records
// the links in the LinkRegistry for later cleanup.
func LinkSkillToAgents(installDir, skillName, fullName string) error {
	agents := agent.DetectAll()
	if len(agents) == 0 {
		output.Warn("No agents detected. Use 'ctx link <agent>' to link manually.")
		return nil
	}

	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}

	for _, a := range agents {
		if err := a.InstallSkill(installDir, skillName); err != nil {
			output.Warn("Failed to link to %s: %v", a.Name(), err)
			continue
		}
		output.PrintDim("  Linked to: %s", a.Name())

		links.Add(fullName, LinkEntry{
			Agent:  a.Name(),
			Type:   LinkSymlink,
			Source: installDir,
			Target: filepath.Join(a.SkillsDir(), skillName),
		})
	}

	links.Save() // best effort
	return nil
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
