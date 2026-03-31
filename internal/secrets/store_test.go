package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_CRUD(t *testing.T) {
	s := New()

	// Set
	s.Set("@test/mcp", "API_KEY", "sk-123")
	s.Set("@test/mcp", "SECRET", "abc")
	s.Set("@other/pkg", "TOKEN", "tok-1")

	// Get
	v, ok := s.Get("@test/mcp", "API_KEY")
	if !ok || v != "sk-123" {
		t.Errorf("Get API_KEY = (%q, %v), want (sk-123, true)", v, ok)
	}

	// Get missing key
	_, ok = s.Get("@test/mcp", "MISSING")
	if ok {
		t.Error("expected missing key to return false")
	}

	// Get missing package
	_, ok = s.Get("@nonexistent/pkg", "KEY")
	if ok {
		t.Error("expected missing package to return false")
	}

	// List
	m := s.List("@test/mcp")
	if len(m) != 2 {
		t.Errorf("List returned %d entries, want 2", len(m))
	}

	// List returns a copy
	m["INJECTED"] = "bad"
	_, injectedOk := s.Get("@test/mcp", "INJECTED")
	if injectedOk {
		t.Error("List returned a reference, not a copy")
	}

	// List nil package
	m = s.List("@nonexistent/pkg")
	if m != nil {
		t.Errorf("List for nonexistent package = %v, want nil", m)
	}

	// Delete
	s.Delete("@test/mcp", "SECRET")
	_, ok = s.Get("@test/mcp", "SECRET")
	if ok {
		t.Error("SECRET should be deleted")
	}
	// API_KEY still exists
	v, ok = s.Get("@test/mcp", "API_KEY")
	if !ok || v != "sk-123" {
		t.Error("API_KEY should still exist")
	}

	// Delete last key removes the package entry
	s.Delete("@test/mcp", "API_KEY")
	m = s.List("@test/mcp")
	if m != nil {
		t.Error("expected package to be removed when last key deleted")
	}

	// Other package unaffected
	v, ok = s.Get("@other/pkg", "TOKEN")
	if !ok || v != "tok-1" {
		t.Error("other package should be unaffected")
	}

	// DeletePackage
	s.Set("@other/pkg", "EXTRA", "val")
	s.DeletePackage("@other/pkg")
	m = s.List("@other/pkg")
	if m != nil {
		t.Error("expected all keys removed after DeletePackage")
	}
}

func TestStore_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CTX_HOME", tmpDir)

	s := New()
	s.Set("@test/mcp", "API_KEY", "sk-456")
	s.Set("@test/mcp", "DB_URL", "postgres://localhost")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file permissions
	p := filepath.Join(tmpDir, secretsFile)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Load
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	v, ok := loaded.Get("@test/mcp", "API_KEY")
	if !ok || v != "sk-456" {
		t.Errorf("loaded API_KEY = (%q, %v), want (sk-456, true)", v, ok)
	}
	v, ok = loaded.Get("@test/mcp", "DB_URL")
	if !ok || v != "postgres://localhost" {
		t.Errorf("loaded DB_URL = (%q, %v)", v, ok)
	}
}

func TestStore_LoadMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CTX_HOME", tmpDir)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load from empty dir: %v", err)
	}
	if len(s.Secrets) != 0 {
		t.Errorf("expected empty store, got %d packages", len(s.Secrets))
	}
}

func TestStore_EmptyNew(t *testing.T) {
	s := New()
	if s.Secrets == nil {
		t.Error("New() should initialize Secrets map")
	}
	if len(s.Secrets) != 0 {
		t.Error("New() should have empty Secrets")
	}
}
