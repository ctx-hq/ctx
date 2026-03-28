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

// agentConstructors is the single source of truth for all known agents.
// Order matters: more specific agents should come before generic.
var agentConstructors = []struct {
	name string
	new  func() Agent
}{
	{"claude", NewClaudeAgent},
	{"cursor", NewCursorAgent},
	{"windsurf", NewWindsurfAgent},
	{"opencode", NewOpenCodeAgent},
	{"codex", NewCodexAgent},
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
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
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
	if err := os.MkdirAll(dst, 0o755); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
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
	return os.WriteFile(configPath, out, 0o644)
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
	return os.WriteFile(configPath, out, 0o644)
}
