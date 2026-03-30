package staging

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Dir represents an atomic staging directory.
// Use New to create, WriteFile to populate, Commit to finalize, Rollback to abort.
type Dir struct {
	Path    string
	cleaned bool
}

// New creates a new temporary staging directory.
func New(prefix string) (*Dir, error) {
	path, err := os.MkdirTemp("", prefix)
	if err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	return &Dir{Path: path}, nil
}

// WriteFile writes data to a file inside the staging directory.
func (d *Dir) WriteFile(name string, data []byte, perm os.FileMode) error {
	if d.cleaned {
		return fmt.Errorf("staging directory already cleaned up")
	}
	target := filepath.Join(d.Path, name)

	// Ensure parent directory exists
	if dir := filepath.Dir(target); dir != d.Path {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}
	}

	return os.WriteFile(target, data, perm)
}

// Commit atomically moves the staging directory to dest.
// If dest already exists, it is renamed to dest + ".bak" first.
// On success, the staging directory is consumed (no need to Rollback).
func (d *Dir) Commit(dest string) error {
	if d.cleaned {
		return fmt.Errorf("staging directory already cleaned up")
	}

	// Ensure parent of dest exists
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create dest parent: %w", err)
	}

	// Backup existing dest
	if _, err := os.Stat(dest); err == nil {
		bak := dest + ".bak"
		// Remove old backup if exists
		_ = os.RemoveAll(bak)
		if err := os.Rename(dest, bak); err != nil {
			return fmt.Errorf("backup existing %s: %w", dest, err)
		}
	}

	// Try atomic rename
	if err := os.Rename(d.Path, dest); err != nil {
		// Cross-device fallback: copy then remove
		if copyErr := copyDir(d.Path, dest); copyErr != nil {
			return fmt.Errorf("move staging to %s: %w (copy fallback also failed: %v)", dest, err, copyErr)
		}
		_ = os.RemoveAll(d.Path)
	}

	d.cleaned = true
	return nil
}

// CopyFrom recursively copies all files from src into the staging directory.
// Existing files in staging with the same name are overwritten.
func (d *Dir) CopyFrom(src string) error {
	if d.cleaned {
		return fmt.Errorf("staging directory already cleaned up")
	}
	return copyDir(src, d.Path)
}

// CopyFiles copies only the specified files/directories from src into the staging directory.
// Paths are relative to src. Missing paths are silently skipped.
func (d *Dir) CopyFiles(src string, paths []string) error {
	if d.cleaned {
		return fmt.Errorf("staging directory already cleaned up")
	}
	for _, p := range paths {
		srcPath := filepath.Join(src, p)
		info, err := os.Stat(srcPath)
		if err != nil {
			continue // skip missing paths
		}
		dstPath := filepath.Join(d.Path, p)
		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("copy %s: %w", p, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return fmt.Errorf("create parent for %s: %w", p, err)
			}
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return fmt.Errorf("copy %s: %w", p, err)
			}
		}
	}
	return nil
}

// NormalizeSkillEntry copies the given skill entry file to root SKILL.md
// so that install-side agent linking works without changes.
// If the source file does not exist in staging, this is a no-op.
func (d *Dir) NormalizeSkillEntry(entry string) error {
	if d.cleaned {
		return fmt.Errorf("staging directory already cleaned up")
	}
	src := filepath.Join(d.Path, entry)
	data, err := os.ReadFile(src)
	if err != nil {
		return nil // source missing — no-op
	}
	return d.WriteFile("SKILL.md", data, 0o644)
}

// Rollback removes the staging directory. Safe to call multiple times.
func (d *Dir) Rollback() {
	if d.cleaned {
		return
	}
	_ = os.RemoveAll(d.Path)
	d.cleaned = true
}

// TarGz creates a tar.gz archive of the staging directory contents and returns
// the opened file as an io.ReadCloser. The caller must close it after use.
// The archive is written to a temp file (not inside staging) so it can be
// streamed to the registry without buffering in memory.
func (d *Dir) TarGz() (io.ReadCloser, error) {
	if d.cleaned {
		return nil, fmt.Errorf("staging directory already cleaned up")
	}

	tmp, err := os.CreateTemp("", "ctx-archive-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temp archive: %w", err)
	}
	tmpPath := tmp.Name()

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	walkErr := filepath.Walk(d.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(d.Path, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}

		header, hErr := tar.FileInfoHeader(info, "")
		if hErr != nil {
			return hErr
		}
		header.Name = rel

		if wErr := tw.WriteHeader(header); wErr != nil {
			return wErr
		}

		if info.IsDir() {
			return nil
		}

		f, fErr := os.Open(path)
		if fErr != nil {
			return fErr
		}
		defer func() { _ = f.Close() }()
		_, copyErr := io.Copy(tw, f)
		return copyErr
	})
	if walkErr != nil {
		_ = tw.Close()
		_ = gw.Close()
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("create archive: %w", walkErr)
	}

	if err := tw.Close(); err != nil {
		_ = gw.Close()
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := gw.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	// Re-open for reading
	opened, err := os.Open(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	return &tempFileReader{File: opened, path: tmpPath}, nil
}

// tempFileReader wraps os.File and removes the file on Close.
type tempFileReader struct {
	*os.File
	path string
}

func (t *tempFileReader) Close() error {
	err := t.File.Close()
	_ = os.Remove(t.path)
	return err
}

// copyDir recursively copies src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// filepath.Walk follows symlinks, so info won't have ModeSymlink set.
		// Use Lstat to detect and skip symlinks.
		linfo, lerr := os.Lstat(path)
		if lerr != nil {
			return lerr
		}
		if linfo.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target, info.Mode())
	})
}

// copyFile copies a single file.
func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
