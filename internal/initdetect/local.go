package initdetect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func detectLocal(ctx context.Context, path string) (*DetectResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", absPath)
	}

	dirName := filepath.Base(absPath)
	result := &DetectResult{
		Kind:    SourceLocal,
		Name:    sanitizeGitHubName(dirName),
		Version: "0.1.0",
	}

	// Read package.json if present
	// For monorepos (workspaces), find the main MCP sub-package
	pj := findMainPackageJSON(absPath)

	// Detect type from files present and the resolved package.json
	pkgType := detectLocalType(absPath, pj)
	result.PackageType = pkgType

	if pj != nil {
		if pj.Description != "" {
			result.Description = pj.Description
		}
		if pj.Version != "" {
			result.Version = pj.Version
		}
		result.License = extractNPMLicense(pj.License)
		result.Keywords = pj.Keywords
		if pj.Homepage != "" {
			result.Homepage = pj.Homepage
		}
		result.Repository = extractNPMRepo(pj.Repository)
		// If the sub-package is an MCP package, set upstream to its npm name
		if pj.MCPName != "" || hasMCPDep(pj) {
			result.PackageType = manifest.TypeMCP
			result.Upstream = &manifest.UpstreamSpec{
				NPM:      pj.Name,
				Tracking: "npm",
			}
			if result.MCP == nil {
				result.MCP = &MCPDetection{
					Transport: "stdio",
					Command:   "npx",
					Args:      []string{"-y", fmt.Sprintf("%s@%s", pj.Name, pj.Version)},
				}
			}
			// Detect Node.js requirement
			if nodeVer, ok := pj.Engines["node"]; ok {
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
	}

	// Read server.json if present (MCP)
	if sjData, err := os.ReadFile(filepath.Join(absPath, "server.json")); err == nil {
		if sj, err := ParseServerJSONBytes(sjData); err == nil {
			if result.Description == "" {
				result.Description = sj.Description
			}
			if sj.Version != "" && sj.Version != "${VERSION}" {
				result.Version = sj.Version
			}
			mcp := &MCPDetection{Transport: "stdio"}
			applyServerJSON(mcp, sj)
			result.MCP = mcp
			result.PackageType = manifest.TypeMCP
		}
	}

	// Type-specific enrichment
	switch result.PackageType {
	case manifest.TypeMCP:
		if result.MCP == nil {
			result.MCP = &MCPDetection{Transport: "stdio"}
		}
	case manifest.TypeSkill:
		// Nothing extra needed
	case manifest.TypeCLI:
		result.CLI = &CLIDetection{
			Binary: dirName,
			Verify: dirName + " --version",
		}
	}

	if result.Description == "" {
		result.Description = "A " + string(result.PackageType) + " package"
	}

	return result, nil
}

func detectLocalType(dir string, pj *npmPackageJSON) manifest.PackageType {
	// Check for server.json (MCP)
	if _, err := os.Stat(filepath.Join(dir, "server.json")); err == nil {
		return manifest.TypeMCP
	}
	// Check for SKILL.md
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return manifest.TypeSkill
	}
	// Check resolved package.json for MCP indicators
	if pj != nil {
		if pj.MCPName != "" {
			return manifest.TypeMCP
		}
		if hasMCPDep(pj) {
			return manifest.TypeMCP
		}
		if strings.Contains(strings.ToLower(pj.Name), "mcp") {
			return manifest.TypeMCP
		}
		if pj.Bin != nil {
			return manifest.TypeCLI
		}
	}
	// Check for goreleaser
	for _, name := range []string{".goreleaser.yaml", ".goreleaser.yml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return manifest.TypeCLI
		}
	}
	// Check for Dockerfile with MCP in name
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		dirLower := strings.ToLower(filepath.Base(dir))
		if strings.Contains(dirLower, "mcp") {
			return manifest.TypeMCP
		}
	}

	return manifest.TypeSkill
}

func readLocalPackageJSON(dir string) (*npmPackageJSON, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}
	var pj npmPackageJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil, err
	}
	return &pj, nil
}

// findMainPackageJSON finds the best package.json for metadata extraction.
// For monorepos with workspaces, it searches sub-packages for MCP indicators.
// For single-package repos, it returns the root package.json.
func findMainPackageJSON(dir string) *npmPackageJSON {
	rootPJ, err := readLocalPackageJSON(dir)
	if err != nil {
		return nil
	}

	// Check if root has workspaces (monorepo)
	if rootPJ.Workspaces == nil || len(rootPJ.Workspaces) == 0 {
		return rootPJ
	}

	// Monorepo: scan workspace members for MCP sub-package
	// Priority: mcpName > MCP SDK dep > name contains "mcp"
	var candidates []*npmPackageJSON
	for _, pattern := range rootPJ.Workspaces {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			subPJ, err := readLocalPackageJSON(match)
			if err != nil {
				continue
			}
			if subPJ.MCPName != "" || hasMCPDep(subPJ) || strings.Contains(strings.ToLower(subPJ.Name), "mcp") {
				candidates = append(candidates, subPJ)
			}
		}
	}

	if len(candidates) > 0 {
		// Sort by specificity: mcpName > MCP SDK dep > name match
		best := candidates[0]
		for _, c := range candidates[1:] {
			if c.MCPName != "" && best.MCPName == "" {
				best = c
			} else if hasMCPDep(c) && !hasMCPDep(best) && best.MCPName == "" {
				best = c
			}
		}
		return best
	}

	// No MCP sub-package found, use root (may have limited metadata)
	return rootPJ
}

func hasMCPDep(pj *npmPackageJSON) bool {
	allDeps := mergeMaps(pj.Dependencies, pj.DevDependencies)
	_, ok := allDeps["@modelcontextprotocol/sdk"]
	return ok
}
