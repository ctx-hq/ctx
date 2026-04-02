package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ctx-hq/ctx/internal/adapter"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
)

// Installer handles downloading and placing packages.
type Installer struct {
	Registry *registry.Client
	Resolver *resolver.Resolver
	DataDir  string
}

// New creates a new installer with registry and resolver for full install operations.
func New(reg *registry.Client, res *resolver.Resolver) *Installer {
	return &Installer{
		Registry: reg,
		Resolver: res,
		DataDir:  config.DataDir(),
	}
}

// NewScanner creates a lightweight installer that can only scan installed packages.
// Use this when you don't need to download or resolve packages.
func NewScanner() *Installer {
	return &Installer{
		DataDir: config.DataDir(),
	}
}

// InstallResult contains the result of an install operation.
type InstallResult struct {
	FullName    string `json:"full_name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	InstallPath string `json:"install_path"`
	Source      string `json:"source"`
	IsNew       bool   `json:"is_new"`
}

// InstallFiles resolves, downloads, extracts, and symlinks a package.
// It is safe to call concurrently for different packages.
func (i *Installer) InstallFiles(ctx context.Context, ref string) (*resolver.Resolution, *manifest.Manifest, error) {
	// Resolve version
	resolution, err := i.Resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve: %w", err)
	}

	// Parse manifest from resolution (may be JSON or YAML depending on source)
	var m manifest.Manifest
	if resolution.Manifest != "" {
		if err := json.Unmarshal([]byte(resolution.Manifest), &m); err != nil {
			// Not valid JSON — try YAML (registry stores manifests as YAML)
			if yamlErr := yaml.Unmarshal([]byte(resolution.Manifest), &m); yamlErr != nil {
				m = manifest.Manifest{
					Name:    resolution.FullName,
					Version: resolution.Version,
					Type:    manifest.TypeSkill,
				}
			}
		}
	}

	// Validate full name to prevent path traversal
	if err := validatePackageName(resolution.FullName); err != nil {
		return nil, nil, fmt.Errorf("invalid package name: %w", err)
	}

	// Versioned storage: ~/.ctx/packages/@scope/name/{version}/
	versionDir := i.VersionDir(resolution.FullName, resolution.Version)
	pkgDir := i.PackageDir(resolution.FullName)

	// Check if this exact version is already installed (with valid manifest)
	versionExists := false
	if _, err := os.Stat(filepath.Join(versionDir, "manifest.json")); err == nil {
		versionExists = true
	}

	// Check for platform-specific artifact and override download URL if available
	downloadURL := resolution.DownloadURL
	isArtifact := false
	artifactSHA256 := ""
	if len(resolution.Artifacts) > 0 {
		currentPlatform := runtime.GOOS + "-" + runtime.GOARCH
		for _, a := range resolution.Artifacts {
			if a.Platform == currentPlatform && a.DownloadURL != "" {
				// API returns relative paths — prepend registry base URL
				if strings.HasPrefix(a.DownloadURL, "/") {
					downloadURL = i.Registry.BaseURL + a.DownloadURL
				} else {
					downloadURL = a.DownloadURL
				}
				artifactSHA256 = a.SHA256
				isArtifact = true
				if artifactSHA256 == "" {
					return nil, nil, fmt.Errorf("artifact for platform %s is missing SHA256 checksum", currentPlatform)
				}
				break
			}
		}
	}

	// Download and extract to versioned directory (skip if version already exists)
	if !versionExists && downloadURL != "" {
		var body io.ReadCloser
		switch resolution.Source {
		case "registry":
			if isArtifact {
				// Platform-specific artifact — only send auth token for registry-hosted URLs
				token := ""
				if strings.HasPrefix(downloadURL, i.Registry.BaseURL) {
					token = i.Registry.Token
				}
				body, err = downloadHTTP(ctx, downloadURL, token)
			} else {
				body, err = i.Registry.Download(ctx, resolution.FullName, resolution.Version)
			}
			if err != nil {
				return nil, nil, fmt.Errorf("download: %w", err)
			}
		case "github":
			body, err = downloadHTTP(ctx, downloadURL, "")
			if err != nil {
				return nil, nil, fmt.Errorf("download from github: %w", err)
			}
		}
		if body != nil {
			defer func() { _ = body.Close() }()

			// For artifact downloads, wrap reader with SHA256 hasher for integrity check
			var hashReader io.Reader = body
			var hasher hash.Hash
			if isArtifact {
				hasher = sha256.New()
				hashReader = io.TeeReader(body, hasher)
			}

			// Extract to a temp dir first, then atomically move to version dir
			if err := os.MkdirAll(pkgDir, 0o700); err != nil {
				return nil, nil, fmt.Errorf("create package dir: %w", err)
			}
			tmpDir, err := os.MkdirTemp(pkgDir, ".ctx-install-*")
			if err != nil {
				return nil, nil, fmt.Errorf("create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }() // clean up on failure

			if err := extractArchive(io.NopCloser(hashReader), tmpDir); err != nil {
				return nil, nil, fmt.Errorf("extract package: %w", err)
			}

			// Verify SHA256 integrity for artifact downloads
			if hasher != nil {
				actualSHA := fmt.Sprintf("%x", hasher.Sum(nil))
				if actualSHA != artifactSHA256 {
					return nil, nil, fmt.Errorf("SHA256 mismatch: expected %s, got %s", artifactSHA256, actualSHA)
				}
			}

			// Write manifest into the temp dir
			manifestData, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("marshal manifest: %w", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), manifestData, 0o644); err != nil {
				return nil, nil, fmt.Errorf("write manifest: %w", err)
			}

			// Atomic move: rename temp to version dir
			if err := os.Rename(tmpDir, versionDir); err != nil {
				return nil, nil, fmt.Errorf("rename to version dir: %w", err)
			}
		}
	} else if !versionExists && downloadURL == "" {
		// No download URL — create version dir with manifest and content
		if err := os.MkdirAll(versionDir, 0o700); err != nil {
			return nil, nil, fmt.Errorf("create version dir: %w", err)
		}
		// Always write manifest so post-install linking can find it
		manifestData, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("marshal manifest: %w", err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "manifest.json"), manifestData, 0o644); err != nil {
			return nil, nil, fmt.Errorf("write manifest: %w", err)
		}
		// For skills without an archive, generate a SKILL.md from the manifest
		if m.Type == manifest.TypeSkill {
			content := generateSkillMD(&m)
			if err := os.WriteFile(filepath.Join(versionDir, "SKILL.md"), []byte(content), 0o644); err != nil {
				return nil, nil, fmt.Errorf("write SKILL.md: %w", err)
			}
		}
	}

	// Switch current symlink to new version
	if err := SwitchCurrent(pkgDir, resolution.Version); err != nil {
		return nil, nil, fmt.Errorf("switch current: %w", err)
	}

	return resolution, &m, nil
}

// Install installs a package from a reference like "@scope/name@^1.0".
func (i *Installer) Install(ctx context.Context, ref string) (*InstallResult, error) {
	// Check if package existed before install (for isNew detection)
	// Parse the ref to get the full name for the check. We extract just
	// the name part before @version, then resolve will give us the real full name.
	// Instead, check after InstallFiles using the resolved full name — but we need
	// to know the state before the version dir is created. Since InstallFiles creates
	// the package directory, we check before calling it. However, we don't know the
	// full name yet. So we check after, using the current symlink: if there was only
	// one version dir (the one we just installed), it's new.
	resolution, m, err := i.InstallFiles(ctx, ref)
	if err != nil {
		return nil, err
	}

	currentPath := i.CurrentLink(resolution.FullName)
	versions := i.InstalledVersions(resolution.FullName)
	isNew := len(versions) == 1 // only one version means this is a fresh install

	result := &InstallResult{
		FullName:    resolution.FullName,
		Version:     resolution.Version,
		Type:        string(m.Type),
		InstallPath: currentPath,
		Source:      resolution.Source,
		IsNew:       isNew,
	}

	// Report telemetry (non-blocking, ignore errors)
	if i.Registry != nil {
		go i.Registry.ReportInstall(context.WithoutCancel(ctx), resolution.FullName, resolution.Version, nil, resolution.Source)
	}

	return result, nil
}

// Remove uninstalls a package, cleaning up links and all version directories.
func (i *Installer) Remove(ctx context.Context, fullName string) error {
	pkgDir := i.PackageDir(fullName)
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return fmt.Errorf("package %s is not installed", fullName)
	}

	// 1. Use state.json to uninstall CLI via the original adapter
	state, _ := installstate.Load(pkgDir)
	if state != nil && state.CLI != nil && state.CLI.Adapter != "" {
		a, aErr := adapter.FindByName(state.CLI.Adapter)
		if aErr == nil {
			_ = a.Uninstall(ctx, state.CLI.AdapterPkg) // best effort
		}
	}

	// 2. Clean up binary symlink in ~/.ctx/bin/
	currentLink := filepath.Join(pkgDir, "current")
	if target, lErr := os.Readlink(currentLink); lErr == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(pkgDir, target)
		}
		manifestPath := filepath.Join(target, "manifest.json")
		if mData, rErr := os.ReadFile(manifestPath); rErr == nil {
			var m manifest.Manifest
			if json.Unmarshal(mData, &m) == nil && m.CLI != nil && m.CLI.Binary != "" {
				binDir := filepath.Join(filepath.Dir(i.DataDir), "bin")
				binLink := filepath.Join(binDir, m.CLI.Binary)
				// Only remove if the symlink points into this package
				if linkTarget, err := os.Readlink(binLink); err == nil && strings.HasPrefix(linkTarget, pkgDir) {
					_ = os.Remove(binLink)
				}
			}
		}
	}

	// 3. Clean up links (symlinks, MCP configs)
	links, linkErr := LoadLinks()
	if linkErr == nil {
		entries := links.Remove(fullName)
		CleanupLinks(entries)
		_ = links.Save() // best effort
	}

	// 4. Remove entire package directory (all versions + current symlink + state.json)
	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("remove package dir: %w", err)
	}

	// Clean up empty parent directories (scope dir)
	cleanEmptyParents(filepath.Dir(pkgDir))

	return nil
}

// downloadHTTP fetches a URL and returns the response body.
func downloadHTTP(ctx context.Context, rawURL, token string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download %s: status %d", rawURL, resp.StatusCode)
	}
	return resp.Body, nil
}

// validatePackageName ensures a package name cannot escape the data directory.
func validatePackageName(name string) error {
	if strings.Contains(name, "..") {
		return fmt.Errorf("package name %q contains path traversal", name)
	}
	cleaned := filepath.Clean(name)
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("package name %q must be relative", name)
	}
	return nil
}

// maxExtractFileSize limits individual file extraction to 500MB.
const maxExtractFileSize = 500 * 1024 * 1024

// extractArchive extracts a tar.gz archive into destDir.
func extractArchive(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Validate that the resolved path stays within destDir
		cleanName := filepath.Clean(hdr.Name)
		if strings.Contains(cleanName, "..") {
			continue
		}
		dest := filepath.Join(destDir, cleanName)
		if !strings.HasPrefix(dest, filepath.Clean(destDir)+string(filepath.Separator)) {
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}

		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755|0o644)
		if err != nil {
			return err
		}
		written, err := io.Copy(f, io.LimitReader(tr, maxExtractFileSize+1))
		if err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
		if written > maxExtractFileSize {
			return fmt.Errorf("file %s exceeds maximum size of %d bytes", hdr.Name, maxExtractFileSize)
		}
	}
	return nil
}

// generateSkillMD creates a minimal SKILL.md from manifest metadata.
func generateSkillMD(m *manifest.Manifest) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + yamlQuote(m.ShortName()) + "\n")
	if m.Description != "" {
		b.WriteString("description: " + yamlQuote(m.Description) + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("# " + m.Name + "\n\n")
	if m.Description != "" {
		b.WriteString(m.Description + "\n")
	}
	return b.String()
}

// yamlQuote wraps a string in double quotes if it contains YAML-significant
// characters (colons, newlines, quotes, leading/trailing whitespace, etc.).
func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := false
	for _, c := range s {
		if c == ':' || c == '#' || c == '\n' || c == '\r' || c == '"' || c == '\'' || c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == '&' || c == '*' || c == '!' || c == '|' || c == '>' || c == '%' || c == '@' || c == '`' {
			needsQuote = true
			break
		}
	}
	if s[0] == ' ' || s[len(s)-1] == ' ' {
		needsQuote = true
	}
	if !needsQuote {
		return s
	}
	// Escape backslashes and double quotes, then wrap in double quotes
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	escaped = strings.ReplaceAll(escaped, "\r", `\r`)
	return `"` + escaped + `"`
}
