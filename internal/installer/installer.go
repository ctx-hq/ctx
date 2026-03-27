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

// Install installs a package from a reference like "@scope/name@^1.0".
func (i *Installer) Install(ctx context.Context, ref string) (*InstallResult, error) {
	// Resolve version
	resolution, err := i.Resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
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
		return nil, fmt.Errorf("invalid package name: %w", err)
	}

	installDir := filepath.Join(i.DataDir, resolution.FullName)

	// Download and extract
	if resolution.DownloadURL != "" {
		var body io.ReadCloser
		switch resolution.Source {
		case "registry":
			body, err = i.Registry.Download(ctx, resolution.FullName, resolution.Version)
			if err != nil {
				return nil, fmt.Errorf("download: %w", err)
			}
		case "github":
			body, err = downloadURL(ctx, resolution.DownloadURL)
			if err != nil {
				return nil, fmt.Errorf("download from github: %w", err)
			}
		}
		if body != nil {
			defer body.Close()

			// Extract to a temp dir first, then atomically swap into place
			// to avoid leaving a dirty state on partial update
			parentDir := filepath.Dir(installDir)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return nil, fmt.Errorf("create parent dir: %w", err)
			}
			tmpDir, err := os.MkdirTemp(parentDir, ".ctx-install-*")
			if err != nil {
				return nil, fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir) // clean up on failure

			if err := extractArchive(body, tmpDir); err != nil {
				return nil, fmt.Errorf("extract package: %w", err)
			}

			// Write manifest into the temp dir
			manifestData, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal manifest: %w", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), manifestData, 0o644); err != nil {
				return nil, fmt.Errorf("write manifest: %w", err)
			}

			// Atomic swap: remove old, rename temp to final
			if err := os.RemoveAll(installDir); err != nil {
				return nil, fmt.Errorf("remove old install dir: %w", err)
			}
			if err := os.Rename(tmpDir, installDir); err != nil {
				return nil, fmt.Errorf("rename temp dir: %w", err)
			}
		}
	} else {
		// No download URL — just ensure the directory exists
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			return nil, fmt.Errorf("create install dir: %w", err)
		}
	}

	// Update lockfile
	lockPath := config.LockFilePath()
	lf, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	isNew := !lf.Has(resolution.FullName)
	lf.Add(LockEntry{
		FullName:    resolution.FullName,
		Version:     resolution.Version,
		Type:        string(m.Type),
		Source:      resolution.Source,
		SHA256:      resolution.SHA256,
		InstallPath: installDir,
	})

	if err := lf.Save(lockPath); err != nil {
		return nil, fmt.Errorf("save lockfile: %w", err)
	}

	return &InstallResult{
		FullName:    resolution.FullName,
		Version:     resolution.Version,
		Type:        string(m.Type),
		InstallPath: installDir,
		Source:      resolution.Source,
		IsNew:       isNew,
	}, nil
}

// Remove uninstalls a package.
func (i *Installer) Remove(ctx context.Context, fullName string) error {
	// Load lockfile
	lockPath := config.LockFilePath()
	lf, err := LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("load lockfile: %w", err)
	}

	entry, ok := lf.Get(fullName)
	if !ok {
		return fmt.Errorf("package %s is not installed", fullName)
	}

	// Update lockfile first to avoid dangling references on partial failure
	lf.Remove(fullName)
	if err := lf.Save(lockPath); err != nil {
		return fmt.Errorf("save lockfile: %w", err)
	}

	// Then remove install directory
	if entry.InstallPath != "" {
		if err := os.RemoveAll(entry.InstallPath); err != nil {
			return fmt.Errorf("remove install dir: %w", err)
		}
	}

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
