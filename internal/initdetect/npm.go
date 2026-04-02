package initdetect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// npmPackageJSON represents relevant fields from the npm registry response.
type npmPackageJSON struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	License     interface{}       `json:"license"` // string or {"type":"MIT"}
	Homepage    string            `json:"homepage"`
	Repository  interface{}       `json:"repository"`
	Keywords    []string          `json:"keywords"`
	Bin         interface{}       `json:"bin"` // string or map[string]string
	DistTags    map[string]string `json:"dist-tags"`
	MCPName     string            `json:"mcpName"`

	// Dependencies used for type detection
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`

	// Engines
	Engines map[string]string `json:"engines"`
}

func detectNPM(ctx context.Context, pkg string) (*DetectResult, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s", pkg)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch npm registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("npm registry returned %d for %s", resp.StatusCode, pkg)
	}

	var meta npmPackageJSON
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode npm response: %w", err)
	}

	// Resolve latest version
	version := meta.Version
	if v, ok := meta.DistTags["latest"]; ok {
		version = v
	}

	// Detect package type
	pkgType := detectNPMType(&meta)

	result := &DetectResult{
		Kind:        SourceNPM,
		PackageType: pkgType,
		Name:        sanitizeNPMName(meta.Name),
		Version:     version,
		Description: meta.Description,
		License:     extractNPMLicense(meta.License),
		Homepage:    meta.Homepage,
		Repository:  extractNPMRepo(meta.Repository),
		Keywords:    meta.Keywords,
		Upstream: &manifest.UpstreamSpec{
			NPM:      meta.Name,
			Tracking: "npm",
		},
	}

	// MCP-specific detection
	if pkgType == manifest.TypeMCP {
		result.MCP = &MCPDetection{
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", fmt.Sprintf("%s@%s", meta.Name, version)},
		}
		// Detect Node.js requirement (npx always needs node)
		if nodeVer, ok := meta.Engines["node"]; ok {
			minVer := extractMinVersion(nodeVer)
			if minVer != "" {
				result.MCP.Require = &manifest.MCPRequireSpec{
					Bins:        []string{"node"},
					MinVersions: map[string]string{"node": minVer},
				}
			} else {
				result.MCP.Require = &manifest.MCPRequireSpec{Bins: []string{"node"}}
			}
		} else {
			result.MCP.Require = &manifest.MCPRequireSpec{Bins: []string{"node"}}
		}
	}

	// CLI-specific detection
	if pkgType == manifest.TypeCLI {
		binary := extractNPMBinary(meta.Bin, meta.Name)
		result.CLI = &CLIDetection{
			Binary: binary,
			Verify: binary + " --version",
			Install: &manifest.InstallSpec{
				Npm: meta.Name,
			},
		}
	}

	return result, nil
}

func detectNPMType(meta *npmPackageJSON) manifest.PackageType {
	// Check for MCP indicators
	if meta.MCPName != "" {
		return manifest.TypeMCP
	}
	allDeps := mergeMaps(meta.Dependencies, meta.DevDependencies)
	if _, ok := allDeps["@modelcontextprotocol/sdk"]; ok {
		return manifest.TypeMCP
	}
	// Check for CLI indicators
	if meta.Bin != nil {
		return manifest.TypeCLI
	}
	// Default to MCP if name contains "mcp"
	if strings.Contains(strings.ToLower(meta.Name), "mcp") {
		return manifest.TypeMCP
	}
	return manifest.TypeSkill
}

func sanitizeNPMName(name string) string {
	// @playwright/mcp -> @mcp/playwright
	// @scope/name -> keep as is
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func extractNPMLicense(v interface{}) string {
	switch l := v.(type) {
	case string:
		return l
	case map[string]interface{}:
		if t, ok := l["type"].(string); ok {
			return t
		}
	}
	return ""
}

func extractNPMRepo(v interface{}) string {
	switch r := v.(type) {
	case string:
		return r
	case map[string]interface{}:
		if url, ok := r["url"].(string); ok {
			// Clean up git+https://... or git://...
			url = strings.TrimPrefix(url, "git+")
			url = strings.TrimSuffix(url, ".git")
			return url
		}
	}
	return ""
}

func extractNPMBinary(bin interface{}, pkgName string) string {
	switch b := bin.(type) {
	case string:
		return b
	case map[string]interface{}:
		// Return first binary name
		for k := range b {
			return k
		}
	}
	// Fallback to package name without scope
	parts := strings.Split(pkgName, "/")
	return parts[len(parts)-1]
}

// extractMinVersion extracts a minimum version from an engines constraint like ">=18" or ">=18.0.0"
func extractMinVersion(constraint string) string {
	constraint = strings.TrimSpace(constraint)
	if strings.HasPrefix(constraint, ">=") {
		v := strings.TrimPrefix(constraint, ">=")
		v = strings.TrimSpace(v)
		// Ensure it's semver-like
		if !strings.Contains(v, ".") {
			v += ".0.0"
		}
		return v
	}
	return ""
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
