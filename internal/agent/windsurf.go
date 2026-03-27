package agent

import (
	"os"
	"path/filepath"
)

type windsurfAgent struct {
	home string
}

func NewWindsurfAgent() Agent {
	home := mustHomeDir()
	return &windsurfAgent{home: home}
}

func (a *windsurfAgent) Name() string { return "windsurf" }

func (a *windsurfAgent) Detected() bool {
	_, err := os.Stat(filepath.Join(a.home, ".windsurf"))
	return err == nil
}

func (a *windsurfAgent) SkillsDir() string {
	return filepath.Join(a.home, ".windsurf", "skills")
}

func (a *windsurfAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *windsurfAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *windsurfAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(a.home, ".windsurf", "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *windsurfAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(a.home, ".windsurf", "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
