package agent

import (
	"os"
	"path/filepath"
)

type claudeAgent struct {
	home string
}

func NewClaudeAgent() Agent {
	home := mustHomeDir()
	return &claudeAgent{home: home}
}

func (a *claudeAgent) Name() string { return "claude" }

func (a *claudeAgent) Detected() bool {
	_, err := os.Stat(filepath.Join(a.home, ".claude"))
	return err == nil
}

func (a *claudeAgent) SkillsDir() string {
	return filepath.Join(a.home, ".claude", "skills")
}

func (a *claudeAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *claudeAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *claudeAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(a.home, ".claude", "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *claudeAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(a.home, ".claude", "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
