package gitutil

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const cmdTimeout = 3 * time.Second

var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)

// RemoteURL returns the HTTPS URL of the "origin" remote for the git
// repository containing dir. Returns "" if dir is not in a git repo,
// has no origin remote, or any error occurs.
// SSH URLs (git@host:owner/repo.git) are converted to HTTPS.
func RemoteURL(dir string) string {
	raw := gitCmd(dir, "remote", "get-url", "origin")
	if raw == "" {
		return ""
	}
	return normalizeGitURL(raw)
}

// Author returns git config user.name for the repository containing dir.
// Returns "" on any error.
func Author(dir string) string {
	return gitCmd(dir, "config", "user.name")
}

// LatestTag returns the latest semver tag (without leading "v") for the
// repository containing dir. Returns "" if no tags exist or any error occurs.
func LatestTag(dir string) string {
	tag := gitCmd(dir, "describe", "--tags", "--abbrev=0")
	tag = strings.TrimPrefix(tag, "v")
	if !semverRegex.MatchString(tag) {
		return ""
	}
	return tag
}

// ChangedDirs returns which of the given memberDirs contain files changed
// between ref and HEAD. A changed file "a/b/c.txt" matches memberDir "a/b".
// If memberDirs is empty, all changed directories (first path component) are returned.
// Returns nil on any git error (e.g., not a git repo).
func ChangedDirs(dir, ref string, memberDirs []string) []string {
	// Use member dirs as path prefixes for git diff to limit scope
	args := []string{"diff", "--name-only", ref, "HEAD"}
	if len(memberDirs) > 0 {
		args = append(args, "--")
		for _, d := range memberDirs {
			args = append(args, d+"/")
		}
	}
	raw := gitCmd(dir, args...)
	if raw == "" {
		return nil
	}

	seen := make(map[string]bool)
	var dirs []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match changed file against known member dirs
		for _, md := range memberDirs {
			if strings.HasPrefix(line, md+"/") || line == md {
				if !seen[md] {
					seen[md] = true
					dirs = append(dirs, md)
				}
				break
			}
		}
	}
	return dirs
}

// gitCmd runs a git command with a timeout and returns trimmed stdout.
func gitCmd(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// normalizeGitURL converts SSH or HTTPS git remote URLs to clean HTTPS URLs.
//
// Supported formats:
//
//	git@github.com:owner/repo.git   → https://github.com/owner/repo
//	ssh://git@github.com/owner/repo → https://github.com/owner/repo
//	https://github.com/owner/repo.git → https://github.com/owner/repo
//	https://github.com/owner/repo    → https://github.com/owner/repo (unchanged)
func normalizeGitURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var url string

	switch {
	// ssh://git@github.com/owner/repo or ssh://git@github.com:22/owner/repo
	case strings.HasPrefix(raw, "ssh://"):
		url = raw
		url = strings.TrimPrefix(url, "ssh://")
		// Remove user@ prefix
		if idx := strings.Index(url, "@"); idx != -1 {
			url = url[idx+1:]
		}
		// Remove port if present (host:22/path → host/path)
		if colonIdx := strings.Index(url, ":"); colonIdx != -1 {
			slashIdx := strings.Index(url, "/")
			if slashIdx != -1 && colonIdx < slashIdx {
				// There's a colon before the first slash — strip port
				url = url[:colonIdx] + url[slashIdx:]
			}
		}
		url = "https://" + url

	// git@github.com:owner/repo.git
	case strings.Contains(raw, "@") && strings.Contains(raw, ":") && !strings.Contains(raw, "://"):
		url = raw
		// Remove user@ prefix
		if idx := strings.Index(url, "@"); idx != -1 {
			url = url[idx+1:]
		}
		// Replace first : with /
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url

	default:
		url = raw
	}

	// Strip trailing .git
	url = strings.TrimSuffix(url, ".git")
	// Strip trailing /
	url = strings.TrimSuffix(url, "/")

	return url
}
