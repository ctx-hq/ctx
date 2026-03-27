package agent

import (
	"path/filepath"
)

// genericAgent uses the .agents/ convention (cross-agent standard).
type genericAgent struct {
	home string
}

func NewGenericAgent() Agent {
	home := mustHomeDir()
	return &genericAgent{home: home}
}

func (a *genericAgent) Name() string { return "generic" }

func (a *genericAgent) Detected() bool {
	// Generic agent is always "detected" as fallback
	return true
}

func (a *genericAgent) SkillsDir() string {
	return filepath.Join(a.home, ".agents", "skills")
}

func (a *genericAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *genericAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *genericAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(a.home, ".agents", "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *genericAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(a.home, ".agents", "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
