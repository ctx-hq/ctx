package initdetect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxREADMESize is the maximum size of a README we'll fetch (1 MB).
const maxREADMESize = 1 << 20

// FetchUpstreamREADME fetches the README from the upstream source.
// Returns nil if not available or on error (best-effort).
func FetchUpstreamREADME(ctx context.Context, result *DetectResult) []byte {
	if result == nil || result.Upstream == nil {
		return nil
	}

	// Try GitHub first (most accurate — raw file from default branch)
	if repo := result.Upstream.GitHub; repo != "" {
		if data := fetchGitHubREADME(ctx, repo); data != nil {
			return data
		}
	}

	// Try npm registry (README field in version metadata)
	if pkg := result.Upstream.NPM; pkg != "" {
		version := result.Version
		if version == "" {
			version = "latest"
		}
		if data := fetchNPMREADME(ctx, pkg, version); data != nil {
			return data
		}
	}

	// Try repository URL as GitHub fallback
	if result.Repository != "" && strings.Contains(result.Repository, "github.com/") {
		// Extract owner/repo from URL
		parts := strings.Split(result.Repository, "github.com/")
		if len(parts) == 2 {
			repo := strings.TrimSuffix(strings.TrimRight(parts[1], "/"), ".git")
			if data := fetchGitHubREADME(ctx, repo); data != nil {
				return data
			}
		}
	}

	return nil
}

func fetchGitHubREADME(ctx context.Context, repo string) []byte {
	req, err := githubRequest(ctx, "GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/README.md", repo))
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	var ghContent struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxREADMESize*2)).Decode(&ghContent); err != nil {
		return nil
	}

	cleaned := strings.NewReplacer("\n", "", "\r", "").Replace(ghContent.Content)
	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil
	}
	return data
}

func fetchNPMREADME(ctx context.Context, pkg, version string) []byte {
	meta, err := fetchNPMVersion(ctx, pkg, version)
	if err != nil {
		return nil
	}
	if meta.Readme == "" || meta.Readme == "ERROR: No README data found!" {
		return nil
	}
	readme := []byte(meta.Readme)
	if len(readme) > maxREADMESize {
		return readme[:maxREADMESize]
	}
	return readme
}
