package adapter

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
)

// maxFileSize limits individual file extraction to 500MB.
const maxFileSize = 500 * 1024 * 1024

// BinaryAdapter downloads and installs a pre-built binary.
type BinaryAdapter struct{}

func (a *BinaryAdapter) Name() string   { return "binary" }
func (a *BinaryAdapter) Available() bool { return true } // always available

func (a *BinaryAdapter) Install(ctx context.Context, rawURL string) error {
	binDir := filepath.Join(config.Dir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", config.UserAgent())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Handle tar.gz archives
	if strings.HasSuffix(rawURL, ".tar.gz") || strings.HasSuffix(rawURL, ".tgz") {
		return extractTarGz(resp.Body, binDir)
	}

	// Handle zip — simplified, just save as-is for now
	if strings.HasSuffix(rawURL, ".zip") {
		return fmt.Errorf("zip extraction not yet implemented — use tar.gz")
	}

	// Plain binary — parse URL to extract clean filename
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	name := filepath.Base(parsedURL.Path)
	if name == "" || name == "." || name == "/" {
		return fmt.Errorf("cannot determine binary name from URL")
	}
	dest := filepath.Join(binDir, name)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func (a *BinaryAdapter) Uninstall(ctx context.Context, binary string) error {
	binDir := filepath.Join(config.Dir(), "bin")
	path := filepath.Join(binDir, binary)
	// Validate the resolved path stays within binDir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	absBinDir, err := filepath.Abs(binDir)
	if err != nil {
		return fmt.Errorf("resolve bin dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absBinDir+string(filepath.Separator)) {
		return fmt.Errorf("invalid binary name: path escapes bin directory")
	}
	return os.Remove(absPath)
}

func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve dest dir: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Only extract regular files, skip directories
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Security: validate the full path before using filepath.Base
		cleanName := filepath.Clean(hdr.Name)
		if strings.Contains(cleanName, "..") {
			continue
		}

		name := filepath.Base(cleanName)
		dest := filepath.Join(absDestDir, name)
		if !strings.HasPrefix(dest, absDestDir+string(filepath.Separator)) {
			continue
		}

		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0o755)
		if err != nil {
			return err
		}
		written, err := io.Copy(f, io.LimitReader(tr, maxFileSize+1))
		if err != nil {
			f.Close()
			return err
		}
		f.Close()
		if written > maxFileSize {
			return fmt.Errorf("file %s exceeds maximum size of %d bytes", hdr.Name, maxFileSize)
		}
	}
	return nil
}
