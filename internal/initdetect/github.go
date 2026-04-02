package initdetect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

type githubRepo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	HTMLURL     string `json:"html_url"`
	License     *struct {
		SPDXID string `json:"spdx_id"`
	} `json:"license"`
	Topics []string `json:"topics"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func detectGitHub(ctx context.Context, repo string) (*DetectResult, error) {
	// Fetch repo metadata
	repoMeta, err := fetchGitHubRepo(ctx, repo)
	if err != nil {
		return nil, err
	}

	result := &DetectResult{
		Kind:        SourceGitHub,
		Name:        sanitizeGitHubName(repoMeta.Name),
		Description: repoMeta.Description,
		Homepage:    repoMeta.Homepage,
		Repository:  repoMeta.HTMLURL,
		Keywords:    repoMeta.Topics,
		Upstream: &manifest.UpstreamSpec{
			GitHub:   repo,
			Tracking: "github-release",
		},
	}

	if repoMeta.License != nil && repoMeta.License.SPDXID != "NOASSERTION" {
		result.License = repoMeta.License.SPDXID
	}

	// Try to get latest release version
	if release, err := fetchGitHubLatestRelease(ctx, repo); err == nil {
		result.Version = strings.TrimPrefix(release.TagName, "v")
	} else {
		result.Version = "0.1.0"
	}

	// Detect type from repo contents
	pkgType := detectGitHubType(ctx, repo, repoMeta)
	result.PackageType = pkgType

	// Type-specific detection
	switch pkgType {
	case manifest.TypeMCP:
		result.MCP = detectGitHubMCP(ctx, repo, result)
	case manifest.TypeCLI:
		result.CLI = &CLIDetection{
			Binary: repoMeta.Name,
			Verify: repoMeta.Name + " --version",
		}
	}

	return result, nil
}

func detectGitHubType(ctx context.Context, repo string, meta *githubRepo) manifest.PackageType {
	// Check topics
	for _, t := range meta.Topics {
		switch t {
		case "mcp-server", "mcp", "model-context-protocol":
			return manifest.TypeMCP
		case "cli", "command-line":
			return manifest.TypeCLI
		}
	}

	// Check for server.json (MCP Registry format)
	if fileExists(ctx, repo, "server.json") {
		return manifest.TypeMCP
	}
	// Check for SKILL.md
	if fileExists(ctx, repo, "SKILL.md") {
		return manifest.TypeSkill
	}
	// Check for Dockerfile + MCP indicators
	if fileExists(ctx, repo, "Dockerfile") {
		if strings.Contains(strings.ToLower(meta.Description), "mcp") ||
			strings.Contains(strings.ToLower(meta.Name), "mcp") {
			return manifest.TypeMCP
		}
	}
	// Check for goreleaser (CLI indicator)
	if fileExists(ctx, repo, ".goreleaser.yaml") || fileExists(ctx, repo, ".goreleaser.yml") {
		return manifest.TypeCLI
	}

	return manifest.TypeSkill
}

func detectGitHubMCP(ctx context.Context, repo string, result *DetectResult) *MCPDetection {
	mcp := &MCPDetection{
		Transport: "stdio",
	}

	// Try to parse server.json for rich metadata
	if sj, err := fetchServerJSON(ctx, repo); err == nil {
		applyServerJSON(mcp, sj)
	}

	// If no command detected from server.json, check for Docker
	if mcp.Command == "" {
		if fileExists(ctx, repo, "Dockerfile") {
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) < 2 {
				return mcp
			}
			image := fmt.Sprintf("ghcr.io/%s/%s", parts[0], parts[1])
			if result.Version != "0.1.0" {
				image += ":" + result.Version
			}
			mcp.Command = "docker"
			mcp.Args = []string{"run", "-i", "--rm", image}
			mcp.Require = &manifest.MCPRequireSpec{Bins: []string{"docker"}}
			result.Upstream.Docker = image
		}
	}

	return mcp
}

func fetchGitHubRepo(ctx context.Context, repo string) (*githubRepo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub repo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, repo)
	}

	var meta githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func fetchGitHubLatestRelease(ctx context.Context, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("no releases found")
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// fileExists checks if a file exists in a GitHub repo using the Contents API.
func fileExists(ctx context.Context, repo, path string) bool {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, path)
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func sanitizeGitHubName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
