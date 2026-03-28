package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/registry"
	"github.com/getctx/ctx/internal/resolver"
)

// Installer handles downloading and placing packages.
type Installer struct {
	Registry *registry.Client
	Resolver *resolver.Resolver
	DataDir  string
}

// New creates a new installer.
func New(reg *registry.Client, res *resolver.Resolver) *Installer {
	return &Installer{
		Registry: reg,
		Resolver: res,
		DataDir:  config.DataDir(),
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

// InstallFiles resolves, downloads, extracts, and symlinks a package without
// updating the lockfile. It is safe to call concurrently for different packages.
// Use UpdateLock afterwards to persist the result to the lockfile.
func (i *Installer) InstallFiles(ctx context.Context, ref string) (*resolver.Resolution, *manifest.Manifest, error) {
	// Resolve version
	resolution, err := i.Resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve: %w", err)
	}

	// Parse manifest from resolution
	var m manifest.Manifest
	if resolution.Manifest != "" {
		if err := json.Unmarshal([]byte(resolution.Manifest), &m); err != nil {
			// If manifest is not valid JSON, treat as source-only install
			m = manifest.Manifest{
				Name:    resolution.FullName,
				Version: resolution.Version,
				Type:    manifest.TypeSkill,
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

	// Check if this exact version is already installed
	versionExists := false
	if _, err := os.Stat(versionDir); err == nil {
		versionExists = true
	}

	// Download and extract to versioned directory (skip if version already exists)
	if !versionExists && resolution.DownloadURL != "" {
		var body io.ReadCloser
		switch resolution.Source {
		case "registry":
			body, err = i.Registry.Download(ctx, resolution.FullName, resolution.Version)
			if err != nil {
				return nil, nil, fmt.Errorf("download: %w", err)
			}
		case "github":
			body, err = downloadURL(ctx, resolution.DownloadURL)
			if err != nil {
				return nil, nil, fmt.Errorf("download from github: %w", err)
			}
		}
		if body != nil {
			defer body.Close()

			// Extract to a temp dir first, then atomically move to version dir
			if err := os.MkdirAll(pkgDir, 0o755); err != nil {
				return nil, nil, fmt.Errorf("create package dir: %w", err)
			}
			tmpDir, err := os.MkdirTemp(pkgDir, ".ctx-install-*")
			if err != nil {
				return nil, nil, fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir) // clean up on failure

			if err := extractArchive(body, tmpDir); err != nil {
				return nil, nil, fmt.Errorf("extract package: %w", err)
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
	} else if !versionExists {
		// No download URL — just ensure the version directory exists
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("create version dir: %w", err)
		}
	}

	// Switch current symlink to new version
	if err := SwitchCurrent(pkgDir, resolution.Version); err != nil {
		return nil, nil, fmt.Errorf("switch current: %w", err)
	}

	return resolution, &m, nil
}

// UpdateLock persists an install result to the lockfile. Not safe for concurrent
// use — callers must serialize access when updating in parallel.
func (i *Installer) UpdateLock(resolution *resolver.Resolution, m *manifest.Manifest) (*InstallResult, error) {
	return i.updateLockAndResult(resolution, m)
}

// Install installs a package from a reference like "@scope/name@^1.0".
func (i *Installer) Install(ctx context.Context, ref string) (*InstallResult, error) {
	resolution, m, err := i.InstallFiles(ctx, ref)
	if err != nil {
		return nil, err
	}
	return i.updateLockAndResult(resolution, m)
}

// updateLockAndResult updates the lockfile and returns the install result.
func (i *Installer) updateLockAndResult(resolution *resolver.Resolution, m *manifest.Manifest) (*InstallResult, error) {
	lockPath := config.LockFilePath()
	lf, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	isNew := !lf.Has(resolution.FullName)
	currentPath := i.CurrentLink(resolution.FullName)
	lf.Add(LockEntry{
		FullName:    resolution.FullName,
		Version:     resolution.Version,
		Type:        string(m.Type),
		Source:      resolution.Source,
		SHA256:      resolution.SHA256,
		InstallPath: currentPath,
	})

	if err := lf.Save(lockPath); err != nil {
		return nil, fmt.Errorf("save lockfile: %w", err)
	}

	return &InstallResult{
		FullName:    resolution.FullName,
		Version:     resolution.Version,
		Type:        string(m.Type),
		InstallPath: currentPath,
		Source:      resolution.Source,
		IsNew:       isNew,
	}, nil
}

// Remove uninstalls a package, cleaning up links, lockfile, and all version directories.
func (i *Installer) Remove(ctx context.Context, fullName string) error {
	// Load lockfile
	lockPath := config.LockFilePath()
	lf, err := LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("load lockfile: %w", err)
	}

	if !lf.Has(fullName) {
		return fmt.Errorf("package %s is not installed", fullName)
	}

	// 1. Clean up links first (symlinks, MCP configs)
	links, linkErr := LoadLinks()
	if linkErr == nil {
		entries := links.Remove(fullName)
		CleanupLinks(entries)
		links.Save() // best effort
	}

	// 2. Update lockfile
	lf.Remove(fullName)
	if err := lf.Save(lockPath); err != nil {
		return fmt.Errorf("save lockfile: %w", err)
	}

	// 3. Remove entire package directory (all versions + current symlink)
	pkgDir := i.PackageDir(fullName)
	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("remove package dir: %w", err)
	}

	// Clean up empty parent directories (scope dir)
	cleanEmptyParents(filepath.Dir(pkgDir))

	return nil
}

// List returns all installed packages.
func (i *Installer) List() ([]LockEntry, error) {
	lockPath := config.LockFilePath()
	lf, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, err
	}
	return lf.List(), nil
}

// downloadURL fetches a URL and returns the response body.
func downloadURL(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
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
	defer gz.Close()

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
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755|0o644)
		if err != nil {
			return err
		}
		written, err := io.Copy(f, io.LimitReader(tr, maxExtractFileSize+1))
		if err != nil {
			f.Close()
			return err
		}
		f.Close()
		if written > maxExtractFileSize {
			return fmt.Errorf("file %s exceeds maximum size of %d bytes", hdr.Name, maxExtractFileSize)
		}
	}
	return nil
}
