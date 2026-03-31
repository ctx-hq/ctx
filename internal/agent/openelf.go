package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// openelfAgent handles OpenELF's custom mcp.json format (StoreData v2).
// OpenELF uses a servers[] array instead of the mcpServers{} map used by
// Claude, Cursor and other agents.
type openelfAgent struct {
	home string
}

// NewOpenELFAgent creates an openelf agent instance.
func NewOpenELFAgent() Agent {
	return &openelfAgent{home: mustHomeDir()}
}

func (a *openelfAgent) Name() string { return "openelf" }

func (a *openelfAgent) Detected() bool {
	_, err := os.Stat(a.baseDir())
	return err == nil
}

func (a *openelfAgent) SkillsDir() string {
	return filepath.Join(a.baseDir(), "skills")
}

func (a *openelfAgent) InstallSkill(srcDir, skillName string) error {
	return installSkillBySymlink(a.SkillsDir(), srcDir, skillName)
}

func (a *openelfAgent) RemoveSkill(skillName string) error {
	return removeSkillDir(a.SkillsDir(), skillName)
}

func (a *openelfAgent) AddMCP(name string, config MCPConfig) error {
	return writeOpenELFMCP(a.mcpPath(), name, config)
}

func (a *openelfAgent) RemoveMCP(name string) error {
	return removeOpenELFMCP(a.mcpPath(), name)
}

// baseDir returns the OpenELF data directory.
// Priority: OPENELF_HOME env > ~/.openelf default.
func (a *openelfAgent) baseDir() string {
	if envHome := os.Getenv("OPENELF_HOME"); envHome != "" {
		return envHome
	}
	return filepath.Join(a.home, ".openelf")
}

func (a *openelfAgent) mcpPath() string {
	return filepath.Join(a.baseDir(), "mcp.json")
}

// writeOpenELFMCP upserts an MCP entry into OpenELF's mcp.json (StoreData v2 format).
func writeOpenELFMCP(configPath, name string, mcpCfg MCPConfig) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}

	// Read existing file into a raw map to preserve unknown fields.
	raw := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read config %s: %w", configPath, err)
	}

	// Ensure version is set.
	if _, ok := raw["version"]; !ok {
		raw["version"] = float64(2)
	}

	// Parse existing servers array.
	var servers []map[string]interface{}
	if rawServers, ok := raw["servers"]; ok {
		if arr, ok := rawServers.([]interface{}); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					servers = append(servers, m)
				}
			}
		}
	}

	// Build the new entry.
	entry := map[string]interface{}{
		"name":    name,
		"enabled": true,
		"source":  "ctx",
	}
	if mcpCfg.Command != "" {
		entry["command"] = mcpCfg.Command
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

	// Upsert: find existing by name or append.
	found := false
	for i, s := range servers {
		sName, ok := s["name"].(string)
		if !ok || sName != name {
			continue
		}
		// Preserve fields we don't manage (timeout, reconnect, headers, etc.).
		for k, v := range entry {
			servers[i][k] = v
		}
		found = true
		break
	}
	if !found {
		servers = append(servers, entry)
	}

	raw["servers"] = servers

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o600)
}

// removeOpenELFMCP removes an MCP entry from OpenELF's mcp.json by name.
func removeOpenELFMCP(configPath, name string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config %s: %w", configPath, err)
	}

	raw := make(map[string]interface{})
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %s: %w", configPath, err)
	}

	rawServers, ok := raw["servers"]
	if !ok {
		return nil
	}
	arr, ok := rawServers.([]interface{})
	if !ok {
		return nil
	}

	filtered := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if sName, ok := m["name"].(string); ok && sName == name {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	if len(filtered) > 0 {
		raw["servers"] = filtered
	} else {
		delete(raw, "servers")
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o600)
}
