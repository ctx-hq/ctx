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

	// Detect type from files present
	pkgType := detectLocalType(absPath)
	result.PackageType = pkgType

	// Read package.json if present
	if pj, err := readLocalPackageJSON(absPath); err == nil {
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

func detectLocalType(dir string) manifest.PackageType {
	// Check for server.json (MCP)
	if _, err := os.Stat(filepath.Join(dir, "server.json")); err == nil {
		return manifest.TypeMCP
	}
	// Check for SKILL.md
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return manifest.TypeSkill
	}
	// Check package.json for MCP indicators
	if pj, err := readLocalPackageJSON(dir); err == nil {
		if pj.MCPName != "" {
			return manifest.TypeMCP
		}
		if _, ok := pj.Dependencies["@modelcontextprotocol/sdk"]; ok {
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
