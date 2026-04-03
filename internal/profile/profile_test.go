package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileStore_SaveLoad_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	store := &ProfileStore{
		Active: "work",
		Profiles: map[string]*Profile{
			"work": {Username: "alice-corp", Registry: "https://registry.getctx.org"},
		},
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Active != "work" {
		t.Errorf("Active = %q, want %q", loaded.Active, "work")
	}
	p, ok := loaded.Profiles["work"]
	if !ok {
		t.Fatal("profile 'work' not found after round-trip")
	}
	if p.Username != "alice-corp" {
		t.Errorf("Username = %q, want %q", p.Username, "alice-corp")
	}
	if p.Registry != "https://registry.getctx.org" {
		t.Errorf("Registry = %q, want %q", p.Registry, "https://registry.getctx.org")
	}
}

func TestProfileStore_SaveLoad_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	store := &ProfileStore{
		Profiles: make(map[string]*Profile),
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Profiles == nil {
		t.Error("Profiles should not be nil after loading empty store")
	}
	if len(loaded.Profiles) != 0 {
		t.Errorf("Profiles length = %d, want 0", len(loaded.Profiles))
	}
}

func TestProfileStore_SaveLoad_MultipleProfiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	store := &ProfileStore{
		Active: "personal",
		Profiles: map[string]*Profile{
			"personal": {Username: "alice", Registry: "https://registry.getctx.org"},
			"work":     {Username: "alice-corp", Registry: "https://registry.getctx.org"},
			"staging":  {Username: "alice", Registry: "https://staging.getctx.org"},
		},
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Profiles) != 3 {
		t.Fatalf("Profiles length = %d, want 3", len(loaded.Profiles))
	}
	for _, name := range []string{"personal", "work", "staging"} {
		if _, ok := loaded.Profiles[name]; !ok {
			t.Errorf("profile %q not found", name)
		}
	}
}

func TestProfileStore_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	store := &ProfileStore{
		Profiles: map[string]*Profile{
			"default": {Username: "test"},
		},
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmp, "profiles.yaml"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"default", false},
		{"work", false},
		{"my-corp", false},
		{"a", false},
		{"a1", false},
		{"a-b", false},
		{"abc-123-def", false},
		{strings.Repeat("a", 64), false},

		{"", true},             // empty
		{"Work", true},         // uppercase
		{"a b", true},          // space
		{"-start", true},       // starts with hyphen
		{"end-", true},         // ends with hyphen
		{"a_b", true},          // underscore
		{"a.b", true},          // dot
		{"a/b", true},          // slash
		{strings.Repeat("a", 65), true}, // too long
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestProfile_RegistryURL(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		p := &Profile{Registry: "https://custom.example.com"}
		if got := p.RegistryURL(); got != "https://custom.example.com" {
			t.Errorf("RegistryURL() = %q, want %q", got, "https://custom.example.com")
		}
	})

	t.Run("without registry", func(t *testing.T) {
		p := &Profile{}
		if got := p.RegistryURL(); got != "https://registry.getctx.org" {
			t.Errorf("RegistryURL() = %q, want default registry", got)
		}
	})
}

func TestLoad_NoProfilesFile_EmptyStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Load() without profiles.yaml triggers migration.
	// The migration may find data in the real keychain, so we just verify
	// that Load() succeeds and returns a valid store.
	store, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if store.Profiles == nil {
		t.Error("Profiles should not be nil")
	}
}

func TestLoad_CorruptedYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Write invalid YAML
	path := filepath.Join(tmp, "profiles.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error for corrupted YAML")
	}
}
