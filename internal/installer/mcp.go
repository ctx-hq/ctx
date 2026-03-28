package installer

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
)

// LinkMCPToAgents configures an MCP server in all detected agents and records
// the config entries in the LinkRegistry for later cleanup.
func LinkMCPToAgents(m *manifest.Manifest) error {
	if m.MCP == nil {
		return fmt.Errorf("package is not an MCP server")
	}

	cfg := agent.MCPConfig{
		Command: m.MCP.Command,
		Args:    m.MCP.Args,
		URL:     m.MCP.URL,
	}
	if len(m.MCP.Env) > 0 {
		cfg.Env = make(map[string]string)
		for _, e := range m.MCP.Env {
			cfg.Env[e.Name] = e.Default
		}
	}

	agents := agent.DetectAll()
	if len(agents) == 0 {
		output.Warn("No agents detected. Use 'ctx link <agent>' to link manually.")
		return nil
	}

	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}

	name := m.ShortName()
	for _, a := range agents {
		if err := a.AddMCP(name, cfg); err != nil {
			output.Warn("Failed to configure %s in %s: %v", name, a.Name(), err)
			continue
		}
		output.PrintDim("  Configured in: %s", a.Name())

		links.Add(m.Name, LinkEntry{
			Agent:     a.Name(),
			Type:      LinkConfig,
			Source:    m.Name,
			ConfigKey: name,
		})
	}

	links.Save() // best effort
	return nil
}

// UnlinkMCPFromAgents removes an MCP config from all detected agents.
func UnlinkMCPFromAgents(name string) error {
	agents := agent.DetectAll()
	for _, a := range agents {
		if err := a.RemoveMCP(name); err != nil {
			output.Warn("Failed to remove %s from %s: %v", name, a.Name(), err)
		}
	}
	return nil
}
