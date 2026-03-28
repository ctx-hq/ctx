package agent

import (
	"os"
	"path/filepath"
)

type codexAgent struct {
	home string
}

func NewCodexAgent() Agent {
	home := mustHomeDir()
	return &codexAgent{home: home}
}

func (a *codexAgent) Name() string { return "codex" }

func (a *codexAgent) Detected() bool {
	// Check CODEX_HOME env first, then default location
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		_, err := os.Stat(codexHome)
		return err == nil
	}
	_, err := os.Stat(filepath.Join(a.home, ".codex"))
	return err == nil
}

func (a *codexAgent) SkillsDir() string {
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.Join(codexHome, "skills")
	}
	return filepath.Join(a.home, ".codex", "skills")
}

func (a *codexAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *codexAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *codexAgent) AddMCP(name string, config MCPConfig) error {
	configPath := filepath.Join(filepath.Dir(a.SkillsDir()), "mcp.json")
	return writeMCPConfig(configPath, name, config)
}

func (a *codexAgent) RemoveMCP(name string) error {
	configPath := filepath.Join(filepath.Dir(a.SkillsDir()), "mcp.json")
	return removeMCPFromConfig(configPath, name)
}
