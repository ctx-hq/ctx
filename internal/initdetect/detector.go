// Package initdetect auto-detects package type and extracts metadata
// from upstream sources (npm, GitHub, Docker, local path) to generate ctx.yaml.
package initdetect

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// SourceKind identifies the upstream source type.
type SourceKind string

const (
	SourceNPM    SourceKind = "npm"
	SourceGitHub SourceKind = "github"
	SourceDocker SourceKind = "docker"
	SourceLocal  SourceKind = "local"
)

// DetectResult holds the outcome of source detection.
type DetectResult struct {
	Kind        SourceKind
	PackageType manifest.PackageType
	Name        string // detected package name
	Version     string
	Description string
	License     string
	Homepage    string
	Repository  string
	Keywords    []string

	// MCP-specific
	MCP *MCPDetection

	// CLI-specific
	CLI *CLIDetection

	// Upstream tracking info
	Upstream *manifest.UpstreamSpec
}

// MCPDetection contains MCP-specific extracted metadata.
type MCPDetection struct {
	Transport string
	Command   string
	Args      []string
	URL       string
	Env       []manifest.EnvVar
	Tools     []string
	Resources []string

	// Additional transports detected
	Transports []manifest.TransportSpec

	// Runtime requirements
	Require *manifest.MCPRequireSpec

	// Post-install hooks
	Hooks *manifest.MCPHooks
}

// CLIDetection contains CLI-specific extracted metadata.
type CLIDetection struct {
	Binary  string
	Verify  string
	Install *manifest.InstallSpec
}

// Detector orchestrates source detection and metadata extraction.
type Detector struct{}

// NewDetector creates a new Detector.
func NewDetector() *Detector {
	return &Detector{}
}

// ParseSource parses a source string into kind and key.
// Formats: "npm:@playwright/mcp", "github:owner/repo", "docker:image", "/local/path"
func ParseSource(source string) (SourceKind, string) {
	if strings.HasPrefix(source, "npm:") {
		return SourceNPM, strings.TrimPrefix(source, "npm:")
	}
	if strings.HasPrefix(source, "github:") {
		return SourceGitHub, strings.TrimPrefix(source, "github:")
	}
	if strings.HasPrefix(source, "docker:") {
		return SourceDocker, strings.TrimPrefix(source, "docker:")
	}
	// Local path (absolute or relative)
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
		return SourceLocal, source
	}
	// Try to infer: if it looks like owner/repo, treat as GitHub
	if strings.Count(source, "/") == 1 && !strings.Contains(source, ":") && !strings.Contains(source, ".") {
		return SourceGitHub, source
	}
	// Default to local
	return SourceLocal, source
}

// Detect analyzes a source and returns detected metadata.
func (d *Detector) Detect(ctx context.Context, source string) (*DetectResult, error) {
	kind, key := ParseSource(source)

	switch kind {
	case SourceNPM:
		return detectNPM(ctx, key)
	case SourceGitHub:
		return detectGitHub(ctx, key)
	case SourceDocker:
		return detectDocker(ctx, key)
	case SourceLocal:
		return detectLocal(ctx, key)
	default:
		return nil, fmt.Errorf("unsupported source kind: %s", kind)
	}
}
