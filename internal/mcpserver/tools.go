package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
)

// Tool definitions for MCP tool discovery.
var toolDefinitions = map[string]map[string]any{
	"ctx_search": {
		"name":        "ctx_search",
		"description": "Search for skills, MCP servers, and CLI tools in the ctx registry",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":    map[string]any{"type": "string", "description": "Search query"},
				"type":     map[string]any{"type": "string", "description": "Filter by type: skill, mcp, cli", "enum": []string{"skill", "mcp", "cli"}},
				"platform": map[string]any{"type": "string", "description": "Filter by platform"},
			},
			"required": []string{"query"},
		},
	},
	"ctx_install": {
		"name":        "ctx_install",
		"description": "Install a skill, MCP server, or CLI tool",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{"type": "string", "description": "Package reference like @scope/name or @scope/name@^1.0"},
			},
			"required": []string{"package"},
		},
	},
	"ctx_info": {
		"name":        "ctx_info",
		"description": "Get detailed information about a package",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{"type": "string", "description": "Package name like @scope/name"},
			},
			"required": []string{"package"},
		},
	},
	"ctx_list": {
		"name":        "ctx_list",
		"description": "List installed packages",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type": map[string]any{"type": "string", "description": "Filter by type: skill, mcp, cli"},
			},
		},
	},
}

// RegisterDefaultTools registers all default ctx tools.
func RegisterDefaultTools(s *Server) {
	s.RegisterTool("ctx_search", handleSearch)
	s.RegisterTool("ctx_install", handleInstall)
	s.RegisterTool("ctx_info", handleInfo)
	s.RegisterTool("ctx_list", handleList)
}

func getToken() string {
	token, err := auth.GetToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return ""
	}
	return token
}

func getClient() (*registry.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return registry.New(cfg.RegistryURL(), getToken()), nil
}

func handleSearch(args json.RawMessage) (any, error) {
	var params struct {
		Query    string `json:"query"`
		Type     string `json:"type"`
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	return client.Search(context.Background(), params.Query, params.Type, params.Platform, 20)
}

func handleInstall(args json.RawMessage) (any, error) {
	var params struct {
		Package string `json:"package"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	res := resolver.New(client)
	inst := installer.New(client, res)

	return inst.Install(context.Background(), params.Package)
}

func handleInfo(args json.RawMessage) (any, error) {
	var params struct {
		Package string `json:"package"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	return client.GetPackage(context.Background(), params.Package)
}

func handleList(args json.RawMessage) (any, error) {
	var params struct {
		Type string `json:"type"`
	}
	json.Unmarshal(args, &params)

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	client := registry.New(cfg.RegistryURL(), getToken())
	res := resolver.New(client)
	inst := installer.New(client, res)

	entries, err := inst.ScanInstalled()
	if err != nil {
		return nil, err
	}

	if params.Type != "" {
		filtered := make([]installer.InstalledPackage, 0)
		for _, e := range entries {
			if e.Type == params.Type {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	return map[string]any{"packages": entries, "total": len(entries)}, nil
}
