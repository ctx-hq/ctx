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
// transportID selects a named transport from mcp.transports[]; empty uses the default top-level transport.
// Returns the list of MCP states for tracking in state.json.
func LinkMCPToAgents(ctx context.Context, m *manifest.Manifest, transportID string) ([]installstate.MCPState, error) {
	w := output.FromContext(ctx)
	w.Verbose(ctx, "configuring MCP %s in detected agents", m.ShortName())
	if m.MCP == nil {
		return nil, fmt.Errorf("package is not an MCP server")
	}

	tid := transportID

	cfg, err := buildMCPConfig(m.MCP, tid)
	if err != nil {
		return nil, err
	}

	// Build env from the selected transport's env vars
	envVars := m.MCP.Env
	if tid != "" {
		for _, t := range m.MCP.Transports {
			if t.ID == tid && len(t.Env) > 0 {
				envVars = t.Env
				break
			}
		}
	}
	if len(envVars) > 0 {
		cfg.Env = make(map[string]string)
		for _, e := range envVars {
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
		w.Warn("load secrets: %v", err)
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
		w.Warn("No agents detected. Use 'ctx link <agent>' to link manually.")
		return nil, nil
	}

	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}

	name := m.ShortName()
	var mcpStates []installstate.MCPState
	var configuredNames []string
	for _, a := range agents {
		if err := a.AddMCP(name, cfg); err != nil {
			w.Warn("Failed to configure %s in %s: %v", name, a.Name(), err)
			mcpStates = append(mcpStates, installstate.MCPState{Agent: a.Name(), ConfigKey: name, Status: "missing"})
			continue
		}

		links.Add(m.Name, LinkEntry{
			Agent:     a.Name(),
			Type:      LinkConfig,
			Source:    m.Name,
			ConfigKey: name,
		})
		mcpStates = append(mcpStates, installstate.MCPState{Agent: a.Name(), ConfigKey: name, Status: "ok", TransportID: tid})
		configuredNames = append(configuredNames, a.Name())
	}

	if len(configuredNames) > 0 {
		w.PrintLinkedAgents(configuredNames)
	}

	_ = links.Save() // best effort
	return mcpStates, nil
}

// buildMCPConfig creates an MCPConfig from the manifest, optionally selecting a named transport.
// Returns an error if a specific transportID is requested but not found.
func buildMCPConfig(mcp *manifest.MCPSpec, transportID string) (agent.MCPConfig, error) {
	if transportID != "" {
		for _, t := range mcp.Transports {
			if t.ID == transportID {
				return agent.MCPConfig{
					Command: t.Command,
					Args:    t.Args,
					URL:     t.URL,
				}, nil
			}
		}
		return agent.MCPConfig{}, fmt.Errorf("transport %q not found in manifest", transportID)
	}
	// Default: use top-level transport
	return agent.MCPConfig{
		Command: mcp.Command,
		Args:    mcp.Args,
		URL:     mcp.URL,
	}, nil
}

// UnlinkMCPFromAgents removes an MCP config from all detected agents.
func UnlinkMCPFromAgents(name string) error {
	w := output.NewWriter()
	agents := agent.DetectAll()
	for _, a := range agents {
		if err := a.RemoveMCP(name); err != nil {
			w.Warn("Failed to remove %s from %s: %v", name, a.Name(), err)
		}
	}
	return nil
}
