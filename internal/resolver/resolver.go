package resolver

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ctx-hq/ctx/internal/registry"
)

// Resolution holds the result of resolving a package reference.
type Resolution struct {
	FullName      string
	Version       string
	Manifest      string // JSON-encoded manifest
	DownloadURL   string
	SHA256        string
	ArchiveSHA256 string // SHA256 of the archive blob for integrity verification
	Source        string // "registry", "github", "local"
	Artifacts     []registry.ArtifactInfo
}

// Resolver resolves package references to installable versions.
type Resolver struct {
	Registry *registry.Client
}

// New creates a new resolver.
func New(reg *registry.Client) *Resolver {
	return &Resolver{Registry: reg}
}

// Resolve resolves a package reference like "@scope/name@^1.0" or "github:user/repo".
func (r *Resolver) Resolve(ctx context.Context, ref string) (*Resolution, error) {
	fullName, constraint := parseRef(ref)

	// GitHub direct install
	if strings.HasPrefix(fullName, "github:") {
		return r.resolveGitHub(ctx, fullName, constraint)
	}

	// Registry resolve
	return r.resolveRegistry(ctx, fullName, constraint)
}

func (r *Resolver) resolveRegistry(ctx context.Context, fullName, constraint string) (*Resolution, error) {
	req := &registry.ResolveRequest{
		Packages: map[string]string{fullName: constraint},
	}
	resp, err := r.Registry.Resolve(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", fullName, err)
	}

	resolved, ok := resp.Resolved[fullName]
	if !ok {
		return nil, fmt.Errorf("package %s not found in resolve response", fullName)
	}

	return &Resolution{
		FullName:      fullName,
		Version:       resolved.Version,
		Manifest:      resolved.Manifest,
		DownloadURL:   resolved.DownloadURL,
		SHA256:        resolved.SHA256,
		ArchiveSHA256: resolved.ArchiveSHA256,
		Source:        "registry",
		Artifacts:     resolved.Artifacts,
	}, nil
}

func (r *Resolver) resolveGitHub(ctx context.Context, source, constraint string) (*Resolution, error) {
	// github:owner/repo[/path][@ref]
	repo := strings.TrimPrefix(source, "github:")
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github source: %s", source)
	}

	owner := parts[0]
	repoName := parts[1]
	var subPath string
	if len(parts) == 3 {
		_, subPath, _ = strings.Cut(parts[1]+"/"+parts[2], "/")
		// Re-parse: first two are owner/repo, rest is path
		repoParts := strings.SplitN(strings.TrimPrefix(source, "github:"), "/", 3)
		owner = repoParts[0]
		repoName = repoParts[1]
		if len(repoParts) == 3 {
			subPath = repoParts[2]
		}
	}

	version := constraint
	if version == "" || version == "*" || version == "latest" {
		version = "main"
	}

	return &Resolution{
		FullName:    fmt.Sprintf("@community/%s", repoName),
		Version:     version,
		Source:      "github",
		DownloadURL: fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.tar.gz", url.PathEscape(owner), url.PathEscape(repoName), url.PathEscape(version)),
		Manifest:    fmt.Sprintf(`{"source":"github:%s/%s","subpath":%q}`, owner, repoName, subPath),
	}, nil
}

// parseRef splits "@scope/name@^1.0" into ("@scope/name", "^1.0").
func parseRef(ref string) (fullName, constraint string) {
	// Handle @scope/name@version
	if strings.HasPrefix(ref, "@") {
		// Find the second @ (version separator)
		rest := ref[1:]
		if idx := strings.Index(rest, "@"); idx != -1 {
			// Check it's after the /
			slashIdx := strings.Index(rest, "/")
			if slashIdx != -1 && idx > slashIdx {
				return ref[:idx+1], rest[idx+1:]
			}
		}
		return ref, "*"
	}

	// Handle name@version without @scope
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		return ref[:idx], ref[idx+1:]
	}

	return ref, "*"
}
