package installer

import (
	"context"
	"fmt"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/secrets"
)

// LinkMCPToAgents configures an MCP server in all detected agents and records
// the config entries in the LinkRegistry for later cleanup.
// Returns the list of MCP states for tracking in state.json.
func LinkMCPToAgents(ctx context.Context, m *manifest.Manifest) ([]installstate.MCPState, error) {
	output.Verbose(ctx, "configuring MCP %s in detected agents", m.ShortName())
	if m.MCP == nil {
		return nil, fmt.Errorf("package is not an MCP server")
	}

	cfg := agent.MCPConfig{
		Command: m.MCP.Command,
		Args:    m.MCP.Args,
		URL:     m.MCP.URL,
	}
	if len(m.MCP.Env) > 0 {
		cfg.Env = make(map[string]string)
		for _, e := range m.MCP.Env {
			// Always include required vars so agents know the key exists.
			// Use "<required>" placeholder when no default is set, to avoid
			// passing an empty string to the MCP server at runtime.
			if e.Required || e.Default != "" {
				v := e.Default
				if v == "" && e.Required {
					v = "<required>"
				}
				cfg.Env[e.Name] = v
			}
		}
	}

	// Overlay with stored secrets (API keys, tokens, etc.)
	if store, err := secrets.Load(); err != nil {
		output.Warn("load secrets: %v", err)
	} else {
		for k, v := range store.List(m.Name) {
			if cfg.Env == nil {
				cfg.Env = make(map[string]string)
			}
			cfg.Env[k] = v
		}
	}

	agents := agent.DetectAll()
	if len(agents) == 0 {
		output.Warn("No agents detected. Use 'ctx link <agent>' to link manually.")
		return nil, nil
	}

	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}

	name := m.ShortName()
	var mcpStates []installstate.MCPState
	for _, a := range agents {
		if err := a.AddMCP(name, cfg); err != nil {
			output.Warn("Failed to configure %s in %s: %v", name, a.Name(), err)
			mcpStates = append(mcpStates, installstate.MCPState{Agent: a.Name(), ConfigKey: name, Status: "missing"})
			continue
		}
		output.PrintDim("  Configured in: %s", a.Name())

		links.Add(m.Name, LinkEntry{
			Agent:     a.Name(),
			Type:      LinkConfig,
			Source:    m.Name,
			ConfigKey: name,
		})
		mcpStates = append(mcpStates, installstate.MCPState{Agent: a.Name(), ConfigKey: name, Status: "ok"})
	}

	_ = links.Save() // best effort
	return mcpStates, nil
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
