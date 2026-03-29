package staging

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Rollback()

	if _, err := os.Stat(d.Path); err != nil {
		t.Fatalf("staging dir should exist: %v", err)
	}
}

func TestWriteFile(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Rollback()

	if err := d.WriteFile("test.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(d.Path, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestWriteFile_NestedDir(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Rollback()

	if err := d.WriteFile("sub/dir/file.txt", []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(d.Path, "sub", "dir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested" {
		t.Errorf("got %q, want %q", string(data), "nested")
	}
}

func TestCommit(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.WriteFile("ctx.yaml", []byte("name: test"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "committed")
	if err := d.Commit(dest); err != nil {
		t.Fatal(err)
	}

	// Staging dir should be gone (renamed)
	if _, err := os.Stat(d.Path); !os.IsNotExist(err) {
		t.Error("staging dir should be removed after commit")
	}

	// Dest should exist with file
	data, err := os.ReadFile(filepath.Join(dest, "ctx.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "name: test" {
		t.Errorf("got %q, want %q", string(data), "name: test")
	}
}

func TestCommit_BackupsExisting(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.WriteFile("new.txt", []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "target")
	// Create existing dir at dest
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Commit(dest); err != nil {
		t.Fatal(err)
	}

	// Backup should exist
	bakData, err := os.ReadFile(filepath.Join(dest+".bak", "old.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bakData) != "old" {
		t.Errorf("backup: got %q, want %q", string(bakData), "old")
	}

	// New content should be at dest
	newData, err := os.ReadFile(filepath.Join(dest, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(newData) != "new" {
		t.Errorf("new: got %q, want %q", string(newData), "new")
	}
}

func TestRollback(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.WriteFile("test.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.Rollback()

	if _, err := os.Stat(d.Path); !os.IsNotExist(err) {
		t.Error("staging dir should be removed after rollback")
	}
}

func TestRollback_DoubleCallSafe(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	d.Rollback()
	d.Rollback() // should not panic
}

func TestCommit_AfterRollback_Errors(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	d.Rollback()

	dest := filepath.Join(t.TempDir(), "dest")
	if err := d.Commit(dest); err == nil {
		t.Error("commit after rollback should error")
	}
}

func TestWriteFile_AfterCleanup_Errors(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	d.Rollback()

	if err := d.WriteFile("test.txt", []byte("data"), 0o644); err == nil {
		t.Error("write after rollback should error")
	}
}

func TestCommit_CreatesParentDirs(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.WriteFile("file.txt", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "deep", "nested", "path")
	if err := d.Commit(dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok" {
		t.Errorf("got %q, want %q", string(data), "ok")
	}
}

func TestTarGz(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Rollback()

	if err := d.WriteFile("ctx.yaml", []byte("name: test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFile("SKILL.md", []byte("# Skill\nContent here."), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, err := d.TarGz()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	// Read the archive and verify contents
	gr, err := gzip.NewReader(rc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	files := make(map[string]string)
	for {
		hdr, readErr := tr.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		data, _ := io.ReadAll(tr)
		files[hdr.Name] = string(data)
	}

	if _, ok := files["ctx.yaml"]; !ok {
		t.Error("archive missing ctx.yaml")
	}
	if content, ok := files["SKILL.md"]; !ok {
		t.Error("archive missing SKILL.md")
	} else if content != "# Skill\nContent here." {
		t.Errorf("SKILL.md content = %q", content)
	}
}

func TestTarGz_AfterCleanup_Errors(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	d.Rollback()

	_, err = d.TarGz()
	if err == nil {
		t.Error("TarGz after rollback should error")
	}
}

func TestTarGz_TempFileCleanedOnClose(t *testing.T) {
	d, err := New("test-staging-")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Rollback()

	if err := d.WriteFile("test.txt", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, err := d.TarGz()
	if err != nil {
		t.Fatal(err)
	}

	// Get the underlying file path
	tfr, ok := rc.(*tempFileReader)
	if !ok {
		t.Fatal("expected tempFileReader")
	}
	tmpPath := tfr.path

	// File should exist before close
	if _, statErr := os.Stat(tmpPath); statErr != nil {
		t.Fatalf("temp file should exist: %v", statErr)
	}

	_ = rc.Close()

	// File should be removed after close
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Error("temp file should be removed after close")
	}
}
