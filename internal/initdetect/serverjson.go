package initdetect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// serverJSON represents the official MCP Registry server.json schema.
// See: https://static.modelcontextprotocol.io/schemas/2025-11-25/server.schema.json
type serverJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Repository  *struct {
		URL string `json:"url"`
	} `json:"repository"`
	Packages []serverJSONPackage `json:"packages"`
	Remotes  []serverJSONRemote  `json:"remotes"`
	Tools    []interface{}       `json:"tools"`
	Env      []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Required    bool   `json:"isRequired"`
		Secret      bool   `json:"isSecret"`
	} `json:"env"`
}

type serverJSONPackage struct {
	RegistryType string `json:"registryType"` // "npm", "oci", etc.
	Identifier   string `json:"identifier"`
	Transport    *struct {
		Type string `json:"type"` // "stdio"
	} `json:"transport"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type serverJSONRemote struct {
	Type string `json:"type"` // "streamable-http"
	URL  string `json:"url"`
}

// fetchServerJSON fetches and parses server.json from a GitHub repo.
func fetchServerJSON(ctx context.Context, repo string) (*serverJSON, error) {
	req, err := githubRequest(ctx, "GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/server.json", repo))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server.json not found")
	}

	// GitHub Contents API returns base64-encoded content
	var ghContent struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghContent); err != nil {
		return nil, err
	}

	// GitHub API returns line-wrapped base64; strip whitespace before decoding.
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, ghContent.Content)
	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	var sj serverJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return nil, fmt.Errorf("parse server.json: %w", err)
	}
	return &sj, nil
}

// applyServerJSON enriches MCPDetection from a parsed server.json.
func applyServerJSON(mcp *MCPDetection, sj *serverJSON) {
	// Extract env vars
	for _, e := range sj.Env {
		mcp.Env = append(mcp.Env, manifest.EnvVar{
			Name:        e.Name,
			Required:    e.Required,
			Description: e.Description,
		})
	}

	// Extract tools
	if len(sj.Tools) > 0 {
		for _, t := range sj.Tools {
			if tm, ok := t.(map[string]interface{}); ok {
				if name, ok := tm["name"].(string); ok {
					mcp.Tools = append(mcp.Tools, name)
				}
			}
		}
	}

	// Extract from packages (primary transport)
	for _, pkg := range sj.Packages {
		if pkg.Transport != nil {
			mcp.Transport = pkg.Transport.Type
		}
		if pkg.Command != "" {
			mcp.Command = pkg.Command
		}
		if len(pkg.Args) > 0 {
			mcp.Args = pkg.Args
		}
		break // use first package
	}

	// Extract remotes as additional transports
	for i, remote := range sj.Remotes {
		id := "remote"
		if len(sj.Remotes) > 1 {
			id = fmt.Sprintf("remote-%d", i)
		}
		mcp.Transports = append(mcp.Transports, manifest.TransportSpec{
			ID:        id,
			Label:     "Remote (" + remote.Type + ")",
			Transport: remote.Type,
			URL:       remote.URL,
		})
	}
}

// ParseServerJSONBytes parses server.json from raw bytes (for local files).
func ParseServerJSONBytes(data []byte) (*serverJSON, error) {
	var sj serverJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return nil, err
	}
	return &sj, nil
}
