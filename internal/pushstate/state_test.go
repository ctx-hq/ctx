package pushstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- HashDir tests ---

func TestHashDir_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ctx.yaml", "name: test\n")
	writeFile(t, dir, "SKILL.md", "# Hello\n")

	h1, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("HashDir not deterministic: %q != %q", h1, h2)
	}
	if h1[:7] != "sha256:" {
		t.Errorf("hash should have sha256: prefix, got %q", h1[:7])
	}
}

func TestHashDir_ContentSensitive(t *testing.T) {
	dir1 := t.TempDir()
	writeFile(t, dir1, "SKILL.md", "version A")

	dir2 := t.TempDir()
	writeFile(t, dir2, "SKILL.md", "version B")

	h1, err := HashDir(dir1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashDir(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestHashDir_FileNameSensitive(t *testing.T) {
	dir1 := t.TempDir()
	writeFile(t, dir1, "a.md", "content")

	dir2 := t.TempDir()
	writeFile(t, dir2, "b.md", "content")

	h1, _ := HashDir(dir1)
	h2, _ := HashDir(dir2)
	if h1 == h2 {
		t.Error("different file names should produce different hashes")
	}
}

func TestHashDir_SkipsIgnoredFiles(t *testing.T) {
	dir1 := t.TempDir()
	writeFile(t, dir1, "SKILL.md", "content")

	dir2 := t.TempDir()
	writeFile(t, dir2, "SKILL.md", "content")
	writeFile(t, dir2, ".DS_Store", "junk")
	writeFile(t, dir2, "package.tar.gz", "archive")
	writeFile(t, dir2, "old.bak", "backup")
	writeFile(t, dir2, "Thumbs.db", "thumbs")

	h1, _ := HashDir(dir1)
	h2, _ := HashDir(dir2)
	if h1 != h2 {
		t.Errorf("ignored files should not affect hash: %q != %q", h1, h2)
	}
}

func TestHashDir_SkipsIgnoredDirs(t *testing.T) {
	dir1 := t.TempDir()
	writeFile(t, dir1, "SKILL.md", "content")

	dir2 := t.TempDir()
	writeFile(t, dir2, "SKILL.md", "content")
	if err := os.MkdirAll(filepath.Join(dir2, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir2, ".git/objects/abc", "git data")
	if err := os.MkdirAll(filepath.Join(dir2, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir2, "node_modules/pkg/index.js", "module.exports = {}")

	h1, _ := HashDir(dir1)
	h2, _ := HashDir(dir2)
	if h1 != h2 {
		t.Errorf("ignored dirs should not affect hash: %q != %q", h1, h2)
	}
}

func TestHashDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	h, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Error("empty dir should produce a valid hash")
	}
}

func TestHashDir_NestedDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "SKILL.md", "skill")
	writeFile(t, dir, "scripts/run.sh", "#!/bin/bash\necho hi")

	h, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Error("nested dir should produce a valid hash")
	}

	// Changing nested file should change hash.
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir2, "SKILL.md", "skill")
	writeFile(t, dir2, "scripts/run.sh", "#!/bin/bash\necho changed")

	h2, _ := HashDir(dir2)
	if h == h2 {
		t.Error("different nested content should produce different hashes")
	}
}

func TestHashDir_Symlinks(t *testing.T) {
	// Symlink to a file should be followed and hashed.
	dir := t.TempDir()
	target := filepath.Join(dir, "real.md")
	writeFile(t, dir, "real.md", "content")
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}

	h, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Error("dir with symlinks should hash successfully")
	}
}

func TestHashDir_NonexistentDir(t *testing.T) {
	_, err := HashDir("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// --- Load/Save tests ---

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("CTX_HOME", t.TempDir())

	ps, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if ps.Version != 1 {
		t.Errorf("Version = %d, want 1", ps.Version)
	}
	if len(ps.Skills) != 0 {
		t.Errorf("Skills should be empty, got %d", len(ps.Skills))
	}
}

func TestLoad_PermissionError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	stateFile := filepath.Join(tmp, stateFileName)
	if err := os.WriteFile(stateFile, []byte(`{"version":1}`), 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		// On some systems (e.g. running as root), permission checks are bypassed.
		// Only assert error when we actually can't read.
		data, readErr := os.ReadFile(stateFile)
		if readErr != nil {
			t.Fatal("expected error for permission-denied file")
		}
		_ = data
	}
}

func TestLoad_CorruptedJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	if err := os.WriteFile(filepath.Join(tmp, stateFileName), []byte("not json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	ps, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps.Skills) != 0 {
		t.Error("corrupted file should return empty state")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	original := &PushState{
		Version: 1,
		Skills: map[string]*SkillState{
			"@test/skill": {
				LastPushedHash: "sha256:abc123",
				LastPushedAt:   time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC),
				LastVersion:    "0.1.0",
				SkillDir:       "/path/to/skill",
			},
		},
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, stateFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	ps, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := ps.Skills["@test/skill"]
	if !ok {
		t.Fatal("expected skill entry")
	}
	if s.LastPushedHash != "sha256:abc123" {
		t.Errorf("hash = %q, want %q", s.LastPushedHash, "sha256:abc123")
	}
	if s.LastVersion != "0.1.0" {
		t.Errorf("version = %q, want %q", s.LastVersion, "0.1.0")
	}
}

func TestSave_Atomic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	ps := &PushState{
		Version: 1,
		Skills: map[string]*SkillState{
			"@test/save": {
				LastPushedHash: "sha256:def456",
				LastVersion:    "1.0.0",
				SkillDir:       "/test",
			},
		},
	}
	if err := ps.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and is valid JSON.
	data, err := os.ReadFile(filepath.Join(tmp, stateFileName))
	if err != nil {
		t.Fatal(err)
	}

	var loaded PushState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if loaded.Skills["@test/save"].LastPushedHash != "sha256:def456" {
		t.Error("saved data mismatch")
	}
}

func TestSave_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	ps := &PushState{Version: 1, Skills: make(map[string]*SkillState)}
	ps.RecordPush("@a/b", "sha256:xxx", "2.0.0", "/skills/a/b")

	if err := ps.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s := loaded.Skills["@a/b"]
	if s == nil {
		t.Fatal("expected skill after roundtrip")
	}
	if s.LastPushedHash != "sha256:xxx" {
		t.Errorf("hash = %q", s.LastPushedHash)
	}
	if s.LastVersion != "2.0.0" {
		t.Errorf("version = %q", s.LastVersion)
	}
}

// --- RecordPush / IsDirty tests ---

func TestRecordPush(t *testing.T) {
	ps := &PushState{Version: 1, Skills: make(map[string]*SkillState)}

	before := time.Now().UTC()
	ps.RecordPush("@scope/name", "sha256:hash1", "0.2.0", "/dir")
	after := time.Now().UTC()

	s := ps.Skills["@scope/name"]
	if s == nil {
		t.Fatal("RecordPush should create entry")
	}
	if s.LastPushedHash != "sha256:hash1" {
		t.Errorf("hash = %q", s.LastPushedHash)
	}
	if s.LastVersion != "0.2.0" {
		t.Errorf("version = %q", s.LastVersion)
	}
	if s.LastPushedAt.Before(before) || s.LastPushedAt.After(after) {
		t.Errorf("timestamp out of range: %v", s.LastPushedAt)
	}
}

func TestRecordPush_OverwritesExisting(t *testing.T) {
	ps := &PushState{Version: 1, Skills: make(map[string]*SkillState)}
	ps.RecordPush("@a/b", "sha256:old", "0.1.0", "/old")
	ps.RecordPush("@a/b", "sha256:new", "0.2.0", "/new")

	s := ps.Skills["@a/b"]
	if s.LastPushedHash != "sha256:new" {
		t.Errorf("expected new hash, got %q", s.LastPushedHash)
	}
	if s.LastVersion != "0.2.0" {
		t.Errorf("expected new version, got %q", s.LastVersion)
	}
}

func TestIsDirty_NeverPushed(t *testing.T) {
	ps := &PushState{Version: 1, Skills: make(map[string]*SkillState)}
	if !ps.IsDirty("@new/skill", "sha256:anything") {
		t.Error("never-pushed skill should be dirty")
	}
}

func TestIsDirty_SameHash(t *testing.T) {
	ps := &PushState{Version: 1, Skills: map[string]*SkillState{
		"@test/clean": {LastPushedHash: "sha256:abc"},
	}}
	if ps.IsDirty("@test/clean", "sha256:abc") {
		t.Error("same hash should not be dirty")
	}
}

func TestIsDirty_DifferentHash(t *testing.T) {
	ps := &PushState{Version: 1, Skills: map[string]*SkillState{
		"@test/mod": {LastPushedHash: "sha256:old"},
	}}
	if !ps.IsDirty("@test/mod", "sha256:new") {
		t.Error("different hash should be dirty")
	}
}

// --- helpers ---

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
