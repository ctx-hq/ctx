package agent

import (
	"os"
	"path/filepath"
)

type opencodeAgent struct {
	home string
}

func NewOpenCodeAgent() Agent {
	home := mustHomeDir()
	return &opencodeAgent{home: home}
}

func (a *opencodeAgent) Name() string { return "opencode" }

func (a *opencodeAgent) Detected() bool {
	_, err := os.Stat(filepath.Join(a.home, ".config", "opencode"))
	return err == nil
}

func (a *opencodeAgent) SkillsDir() string {
	return filepath.Join(a.home, ".config", "opencode", "skill")
}

func (a *opencodeAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *opencodeAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *opencodeAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(a.home, ".config", "opencode", "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *opencodeAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(a.home, ".config", "opencode", "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
