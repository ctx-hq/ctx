package agent

import (
	"os"
	"path/filepath"
)

type cursorAgent struct {
	home string
}

func NewCursorAgent() Agent {
	home := mustHomeDir()
	return &cursorAgent{home: home}
}

func (a *cursorAgent) Name() string { return "cursor" }

func (a *cursorAgent) Detected() bool {
	_, err := os.Stat(filepath.Join(a.home, ".cursor"))
	return err == nil
}

func (a *cursorAgent) SkillsDir() string {
	return filepath.Join(a.home, ".cursor", "skills")
}

func (a *cursorAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *cursorAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *cursorAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(a.home, ".cursor", "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *cursorAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(a.home, ".cursor", "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
