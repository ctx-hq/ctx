package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// mustHomeDir returns the user home directory or panics if unavailable.
func mustHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		if home = os.Getenv("HOME"); home == "" {
			panic("cannot determine home directory: " + err.Error())
		}
	}
	return home
}

// Agent represents an AI coding agent that ctx can link packages to.
type Agent interface {
	Name() string
	Detected() bool
	SkillsDir() string                              // Where skills are installed
	InstallSkill(srcDir, skillName string) error     // Copy/symlink a skill
	RemoveSkill(skillName string) error
	AddMCP(name string, config MCPConfig) error      // Add MCP to agent config
	RemoveMCP(name string) error
}

// MCPConfig is the MCP server configuration written to agent configs.
type MCPConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// simpleAgent covers all agents whose behaviour differs only in name and paths.
type simpleAgent struct {
	name         string
	home         string
	configDir    string // relative to home, e.g. ".claude" or ".config/zed"
	skillsSubdir string // subdirectory under configDir for skills; defaults to "skills"
}

func newSimpleAgent(name, configDir string) Agent {
	return &simpleAgent{name: name, home: mustHomeDir(), configDir: configDir, skillsSubdir: "skills"}
}

func newSimpleAgentWithSkillsDir(name, configDir, skillsSubdir string) Agent {
	return &simpleAgent{name: name, home: mustHomeDir(), configDir: configDir, skillsSubdir: skillsSubdir}
}

func (a *simpleAgent) Name() string { return a.name }

func (a *simpleAgent) Detected() bool {
	_, err := os.Stat(filepath.Join(a.home, a.configDir))
	return err == nil
}

func (a *simpleAgent) SkillsDir() string {
	return filepath.Join(a.home, a.configDir, a.skillsSubdir)
}

func (a *simpleAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *simpleAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *simpleAgent) AddMCP(name string, config MCPConfig) error {
	return writeMCPConfig(filepath.Join(a.home, a.configDir, "mcp.json"), name, config)
}

func (a *simpleAgent) RemoveMCP(name string) error {
	return removeMCPFromConfig(filepath.Join(a.home, a.configDir, "mcp.json"), name)
}

// agentConstructors is the single source of truth for all known agents.
// Order matters: more specific agents should come before generic.
var agentConstructors = []struct {
	name string
	new  func() Agent
}{
	{"claude", func() Agent { return newSimpleAgent("claude", ".claude") }},
	{"cursor", func() Agent { return newSimpleAgent("cursor", ".cursor") }},
	{"windsurf", func() Agent { return newSimpleAgent("windsurf", ".windsurf") }},
	{"opencode", func() Agent { return newSimpleAgentWithSkillsDir("opencode", ".config/opencode", "skill") }},
	{"codex", NewCodexAgent},
	{"copilot", func() Agent { return newSimpleAgent("copilot", ".config/github-copilot") }},
	{"cline", func() Agent { return newSimpleAgent("cline", ".cline") }},
	{"continue", func() Agent { return newSimpleAgent("continue", ".continue") }},
	{"zed", func() Agent { return newSimpleAgent("zed", ".config/zed") }},
	{"roo", func() Agent { return newSimpleAgent("roo", ".roo-code") }},
	{"goose", func() Agent { return newSimpleAgent("goose", ".config/goose") }},
	{"amp", func() Agent { return newSimpleAgent("amp", ".amp") }},
	{"trae", func() Agent { return newSimpleAgent("trae", ".trae") }},
	{"kilo", func() Agent { return newSimpleAgent("kilo", ".kilo-code") }},
	{"pear", func() Agent { return newSimpleAgent("pear", ".pear-ai") }},
	{"junie", func() Agent { return newSimpleAgent("junie", ".junie") }},
	{"openelf", NewOpenELFAgent},
	{"aider", func() Agent { return newSimpleAgent("aider", ".aider") }},
	{"generic", NewGenericAgent},
}

// DetectAll finds all installed agents on the system.
func DetectAll() []Agent {
	var detected []Agent
	for _, ac := range agentConstructors {
		a := ac.new()
		if a.Detected() {
			detected = append(detected, a)
		}
	}
	return detected
}

// FindByName returns a specific agent by name.
func FindByName(name string) (Agent, error) {
	for _, ac := range agentConstructors {
		if ac.name == name {
			return ac.new(), nil
		}
	}
	names := make([]string, len(agentConstructors))
	for i, ac := range agentConstructors {
		names[i] = ac.name
	}
	return nil, fmt.Errorf("unknown agent: %s (available: %s)", name, strings.Join(names, ", "))
}

// installSkillBySymlink creates a symlink from the agent's skills dir to the source.
// Falls back to copying with a .ctx-managed marker if symlink creation fails
// (e.g., on Windows without developer mode, or cross-filesystem).
func installSkillBySymlink(skillsDir, srcDir, skillName string) error {
	if err := os.MkdirAll(skillsDir, 0o700); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	target := filepath.Join(skillsDir, skillName)
	// Remove existing if present (use RemoveAll in case it's a directory)
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove existing skill: %w", err)
	}

	// Try symlink first
	if err := os.Symlink(srcDir, target); err == nil {
		return nil
	}

	// Fallback: copy directory with .ctx-managed marker
	return copyDirWithMarker(srcDir, target)
}

// copyDirWithMarker copies a directory and writes a .ctx-managed marker.
func copyDirWithMarker(src, dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirWithMarker(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}

	// Write marker to indicate ctx manages this copy
	return os.WriteFile(filepath.Join(dst, ".ctx-managed"),
		[]byte("managed by ctx - do not edit manually\n"), 0o644)
}

// removeSkillDir removes a skill from the skills directory.
func removeSkillDir(skillsDir, skillName string) error {
	target := filepath.Join(skillsDir, skillName)
	return os.RemoveAll(target)
}

// writeMCPConfig reads an MCP config JSON, adds an entry, and writes it back.
func writeMCPConfig(configPath, name string, mcpCfg MCPConfig) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}

	existing := make(map[string]any)
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read config %s: %w", configPath, err)
	}

	servers, _ := existing["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
	}

	entry := map[string]any{
		"command": mcpCfg.Command,
	}
	if len(mcpCfg.Args) > 0 {
		entry["args"] = mcpCfg.Args
	}
	if len(mcpCfg.Env) > 0 {
		entry["env"] = mcpCfg.Env
	}
	if mcpCfg.URL != "" {
		entry["url"] = mcpCfg.URL
	}

	servers[name] = entry
	existing["mcpServers"] = servers

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o600)
}

// removeMCPFromConfig removes an MCP entry from a config file.
func removeMCPFromConfig(configPath, name string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // file doesn't exist, nothing to remove
		}
		return fmt.Errorf("read config %s: %w", configPath, err)
	}

	existing := make(map[string]any)
	if err := json.Unmarshal(data, &existing); err != nil {
		return err
	}

	servers, _ := existing["mcpServers"].(map[string]any)
	if servers != nil {
		delete(servers, name)
		existing["mcpServers"] = servers
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o600)
}
