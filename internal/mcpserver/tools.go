package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/doctor"
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
				"limit":    map[string]any{"type": "integer", "description": "Max results (default 20, max 100)"},
				"offset":   map[string]any{"type": "integer", "description": "Offset for pagination (default 0)"},
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
	"ctx_remove": {
		"name":        "ctx_remove",
		"description": "Remove an installed package",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{"type": "string", "description": "Package name like @scope/name"},
			},
			"required": []string{"package"},
		},
	},
	"ctx_update": {
		"name":        "ctx_update",
		"description": "Update installed packages to their latest versions",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{"type": "string", "description": "Package to update (optional, omit to update all)"},
			},
		},
	},
	"ctx_outdated": {
		"name":        "ctx_outdated",
		"description": "Check which installed packages have newer versions available",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type": map[string]any{"type": "string", "description": "Filter by type: skill, mcp, cli"},
			},
		},
	},
	"ctx_doctor": {
		"name":        "ctx_doctor",
		"description": "Run diagnostic checks on the ctx environment (config, auth, registry, agents, packages)",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{},
		},
	},
	"ctx_agents": {
		"name":        "ctx_agents",
		"description": "List detected AI agents and their skill/MCP installation status",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{},
		},
	},
}

// RegisterDefaultTools registers all default ctx tools.
func RegisterDefaultTools(s *Server) {
	s.RegisterTool("ctx_search", handleSearch)
	s.RegisterTool("ctx_install", handleInstall)
	s.RegisterTool("ctx_info", handleInfo)
	s.RegisterTool("ctx_list", handleList)
	s.RegisterTool("ctx_remove", handleRemove)
	s.RegisterTool("ctx_update", handleUpdate)
	s.RegisterTool("ctx_outdated", handleOutdated)
	s.RegisterTool("ctx_doctor", handleDoctor)
	s.RegisterTool("ctx_agents", handleAgents)
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

func handleSearch(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Query    string `json:"query"`
		Type     string `json:"type"`
		Platform string `json:"platform"`
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	return client.SearchWithOffset(ctx, params.Query, params.Type, params.Platform, limit, params.Offset)
}

func handleInstall(ctx context.Context, args json.RawMessage) (any, error) {
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

	return inst.Install(ctx, params.Package)
}

func handleInfo(ctx context.Context, args json.RawMessage) (any, error) {
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

	return client.GetPackage(ctx, params.Package)
}

func handleRemove(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Package string `json:"package"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Package == "" {
		return nil, fmt.Errorf("package is required")
	}

	client, err := getClient()
	if err != nil {
		return nil, err
	}

	res := resolver.New(client)
	inst := installer.New(client, res)

	if err := inst.Remove(ctx, params.Package); err != nil {
		return nil, err
	}

	return map[string]string{"removed": params.Package}, nil
}

func handleUpdate(ctx context.Context, args json.RawMessage) (any, error) {
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

	// If specific package, verify it's installed before updating
	if params.Package != "" {
		entries, scanErr := inst.ScanInstalled()
		if scanErr != nil {
			return nil, scanErr
		}
		found := false
		for _, e := range entries {
			if e.FullName == params.Package {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("package %s is not installed; use ctx_install instead", params.Package)
		}
		result, installErr := inst.Install(ctx, params.Package)
		if installErr != nil {
			return nil, installErr
		}
		return result, nil
	}

	// Update all: scan → resolve → install
	entries, err := inst.ScanInstalled()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return map[string]any{"updated": 0, "message": "no packages installed"}, nil
	}

	type updateResult struct {
		FullName   string `json:"full_name"`
		OldVersion string `json:"old_version"`
		NewVersion string `json:"new_version"`
		Updated    bool   `json:"updated"`
		Error      string `json:"error,omitempty"`
	}

	var results []updateResult
	for _, e := range entries {
		// Check latest version first to avoid unnecessary reinstall
		pkg, getErr := client.GetPackage(ctx, e.FullName)
		if getErr != nil {
			results = append(results, updateResult{
				FullName:   e.FullName,
				OldVersion: e.Version,
				NewVersion: e.Version,
				Error:      getErr.Error(),
			})
			continue
		}
		if pkg.Version == "" || pkg.Version == e.Version {
			results = append(results, updateResult{
				FullName:   e.FullName,
				OldVersion: e.Version,
				NewVersion: e.Version,
			})
			continue
		}

		result, installErr := inst.Install(ctx, e.FullName)
		if installErr != nil {
			results = append(results, updateResult{
				FullName:   e.FullName,
				OldVersion: e.Version,
				NewVersion: e.Version,
				Error:      installErr.Error(),
			})
			continue
		}
		results = append(results, updateResult{
			FullName:   e.FullName,
			OldVersion: e.Version,
			NewVersion: result.Version,
			Updated:    result.Version != e.Version,
		})
	}

	return results, nil
}

func handleOutdated(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Type string `json:"type"`
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

	entries, err := inst.ScanInstalled()
	if err != nil {
		return nil, err
	}

	// Filter by type if specified
	if params.Type != "" {
		filtered := make([]installer.InstalledPackage, 0)
		for _, e := range entries {
			if e.Type == params.Type {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	type outdatedEntry struct {
		FullName   string `json:"full_name"`
		Current    string `json:"current"`
		Latest     string `json:"latest"`
		HasUpdate  bool   `json:"has_update"`
		Type       string `json:"type"`
	}

	var results []outdatedEntry
	for _, e := range entries {
		pkg, getErr := client.GetPackage(ctx, e.FullName)
		if getErr != nil {
			continue
		}
		hasUpdate := pkg.Version != "" && pkg.Version != e.Version
		results = append(results, outdatedEntry{
			FullName:  e.FullName,
			Current:   e.Version,
			Latest:    pkg.Version,
			HasUpdate: hasUpdate,
			Type:      e.Type,
		})
	}

	return results, nil
}

func handleDoctor(_ context.Context, _ json.RawMessage) (any, error) {
	return doctor.RunChecks("mcp-server", getToken()), nil
}

func handleAgents(_ context.Context, _ json.RawMessage) (any, error) {
	detected := agent.DetectAll()

	type agentInfo struct {
		Name       string `json:"name"`
		Detected   bool   `json:"detected"`
		SkillsDir  string `json:"skills_dir"`
		SkillCount int    `json:"skill_count"`
	}

	var results []agentInfo
	for _, a := range detected {
		skillCount := 0
		skillsDir := a.SkillsDir()
		if entries, err := os.ReadDir(skillsDir); err == nil {
			skillCount = len(entries)
		}
		results = append(results, agentInfo{
			Name:       a.Name(),
			Detected:   true,
			SkillsDir:  skillsDir,
			SkillCount: skillCount,
		})
	}

	return map[string]any{"agents": results, "total": len(results)}, nil
}

func handleList(_ context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

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
