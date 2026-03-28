package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileKeychain_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}

	// Set
	if err := kc.Set("test-svc", "test-acct", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get
	val, err := kc.Get("test-svc", "test-acct")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "secret123" {
		t.Errorf("Get = %q, want %q", val, "secret123")
	}

	// Delete
	if err := kc.Delete("test-svc", "test-acct"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get after delete should error
	_, err = kc.Get("test-svc", "test-acct")
	if err == nil {
		t.Error("Get after Delete should return error")
	}
}

func TestFileKeychain_Permissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}
	if err := kc.Set("test-svc", "test-acct", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	path := filepath.Join(tmp, "credentials")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file perm = %o, want 0600", perm)
	}
}

func TestFileKeychain_UpdateExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}
	_ = kc.Set("test-svc", "test-acct", "old-secret")
	_ = kc.Set("test-svc", "test-acct", "new-secret")

	val, err := kc.Get("test-svc", "test-acct")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "new-secret" {
		t.Errorf("Get = %q, want %q", val, "new-secret")
	}
}

func TestFileKeychain_MultipleEntries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}
	_ = kc.Set("svc1", "acct1", "secret1")
	kc.Set("svc2", "acct2", "secret2")

	v1, _ := kc.Get("svc1", "acct1")
	v2, _ := kc.Get("svc2", "acct2")

	if v1 != "secret1" {
		t.Errorf("svc1 = %q, want secret1", v1)
	}
	if v2 != "secret2" {
		t.Errorf("svc2 = %q, want secret2", v2)
	}

	// Delete one, other survives
	_ = kc.Delete("svc1", "acct1")
	v2, _ = kc.Get("svc2", "acct2")
	if v2 != "secret2" {
		t.Errorf("svc2 after delete svc1 = %q, want secret2", v2)
	}
}

func TestFileKeychain_GetMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}
	_, err := kc.Get("nonexistent", "nonexistent")
	if err == nil {
		t.Error("Get nonexistent should return error")
	}
}

func TestFileKeychain_DeleteMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	kc := &fileKeychain{}
	// Should not error when deleting nonexistent
	if err := kc.Delete("nonexistent", "nonexistent"); err != nil {
		t.Errorf("Delete nonexistent should not error: %v", err)
	}
}

func TestGetKeychain_FallbackToFile(t *testing.T) {
	// When defaultKeychain is nil, should return fileKeychain
	saved := defaultKeychain
	defaultKeychain = nil
	defer func() { defaultKeychain = saved }()

	kc := getKeychain()
	if _, ok := kc.(*fileKeychain); !ok {
		t.Errorf("getKeychain() should return *fileKeychain when default is nil, got %T", kc)
	}
}
