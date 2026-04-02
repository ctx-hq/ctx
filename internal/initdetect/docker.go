package initdetect

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func detectDocker(ctx context.Context, image string) (*DetectResult, error) {
	// Parse image name: ghcr.io/github/github-mcp-server[:tag]
	name := image
	version := "latest"

	// Split off tag
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		// Make sure it's a tag, not a port (check if after : has no /)
		after := image[idx+1:]
		if !strings.Contains(after, "/") {
			version = after
			name = image[:idx]
		}
	}

	// Extract short name from image path
	parts := strings.Split(name, "/")
	shortName := parts[len(parts)-1]

	result := &DetectResult{
		Kind:        SourceDocker,
		PackageType: manifest.TypeMCP,
		Name:        sanitizeGitHubName(shortName),
		Version:     strings.TrimPrefix(version, "v"),
		Description: fmt.Sprintf("MCP server from Docker image %s", name),
		Upstream: &manifest.UpstreamSpec{
			Docker:   name,
			Tracking: "docker",
		},
		MCP: &MCPDetection{
			Transport: "stdio",
			Command:   "docker",
			Args:      []string{"run", "-i", "--rm", image},
			Require: &manifest.MCPRequireSpec{
				Bins: []string{"docker"},
			},
		},
	}

	// Try to detect GitHub repo from image path (ghcr.io/owner/name -> owner/name)
	if strings.HasPrefix(name, "ghcr.io/") {
		ghParts := strings.SplitN(strings.TrimPrefix(name, "ghcr.io/"), "/", 2)
		if len(ghParts) == 2 {
			result.Repository = fmt.Sprintf("https://github.com/%s/%s", ghParts[0], ghParts[1])
			result.Upstream.GitHub = ghParts[0] + "/" + ghParts[1]
		}
	}

	return result, nil
}
