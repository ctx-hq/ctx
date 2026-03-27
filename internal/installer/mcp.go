package installer

import (
	"fmt"

	"github.com/getctx/ctx/internal/agent"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
)

// LinkMCPToAgents configures an MCP server in all detected agents.
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

	name := m.ShortName()
	for _, a := range agents {
		if err := a.AddMCP(name, cfg); err != nil {
			output.Warn("Failed to configure %s in %s: %v", name, a.Name(), err)
			continue
		}
		output.PrintDim("  Configured in: %s", a.Name())
	}
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
