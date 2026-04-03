package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_FlagHighestPriority(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "env-profile")

	store := &ProfileStore{
		Active: "global-profile",
		Profiles: map[string]*Profile{
			"flag-profile":   {Username: "flag-user"},
			"env-profile":    {Username: "env-user"},
			"global-profile": {Username: "global-user"},
		},
	}

	res, err := resolveFromStore(store, "flag-profile")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "flag-profile" {
		t.Errorf("Name = %q, want %q", res.Name, "flag-profile")
	}
	if res.Source != "flag" {
		t.Errorf("Source = %q, want %q", res.Source, "flag")
	}
	if res.Profile.Username != "flag-user" {
		t.Errorf("Username = %q, want %q", res.Profile.Username, "flag-user")
	}
}

func TestResolve_EnvOverridesProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "env-profile")

	// Create .ctx-profile in CWD
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("project-profile"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := &ProfileStore{
		Active: "global-profile",
		Profiles: map[string]*Profile{
			"env-profile":     {Username: "env-user"},
			"project-profile": {Username: "project-user"},
			"global-profile":  {Username: "global-user"},
		},
	}

	res, err := resolveFromStore(store, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "env-profile" {
		t.Errorf("Name = %q, want %q", res.Name, "env-profile")
	}
	if res.Source != "env" {
		t.Errorf("Source = %q, want %q", res.Source, "env")
	}
}

func TestResolve_ProjectOverridesGlobal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "") // clear

	// Create .ctx-profile in a temp dir and chdir to it
	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".ctx-profile"), []byte("project-profile"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldDir, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(oldDir)

	store := &ProfileStore{
		Active: "global-profile",
		Profiles: map[string]*Profile{
			"project-profile": {Username: "project-user"},
			"global-profile":  {Username: "global-user"},
		},
	}

	res, err := resolveFromStore(store, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "project-profile" {
		t.Errorf("Name = %q, want %q", res.Name, "project-profile")
	}
	if res.Source != "project" {
		t.Errorf("Source = %q, want %q", res.Source, "project")
	}
}

func TestResolve_GlobalActive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "")

	// Chdir to a dir without .ctx-profile
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	store := &ProfileStore{
		Active: "global-profile",
		Profiles: map[string]*Profile{
			"global-profile": {Username: "global-user"},
		},
	}

	res, err := resolveFromStore(store, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "global-profile" {
		t.Errorf("Name = %q, want %q", res.Name, "global-profile")
	}
	if res.Source != "global" {
		t.Errorf("Source = %q, want %q", res.Source, "global")
	}
}

func TestResolve_DefaultFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "")

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	store := &ProfileStore{
		Profiles: map[string]*Profile{
			"default": {Username: "default-user"},
		},
	}

	res, err := resolveFromStore(store, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "default" {
		t.Errorf("Name = %q, want %q", res.Name, "default")
	}
	if res.Source != "default" {
		t.Errorf("Source = %q, want %q", res.Source, "default")
	}
}

func TestResolve_NoProfiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "")

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	store := &ProfileStore{
		Profiles: make(map[string]*Profile),
	}

	_, err := resolveFromStore(store, "")
	if !errors.Is(err, ErrNoProfile) {
		t.Errorf("error = %v, want ErrNoProfile", err)
	}
}

func TestResolve_DeletedActiveProfile_WithDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "")

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	store := &ProfileStore{
		Active: "deleted",
		Profiles: map[string]*Profile{
			"default": {Username: "default-user"},
		},
	}

	res, err := resolveFromStore(store, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Name != "default" {
		t.Errorf("Name = %q, want %q", res.Name, "default")
	}
}

func TestResolve_DeletedActiveProfile_NoDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	t.Setenv("CTX_PROFILE", "")

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	store := &ProfileStore{
		Active: "deleted",
		Profiles: map[string]*Profile{
			"other": {Username: "other-user"},
		},
	}

	_, err := resolveFromStore(store, "")
	if !errors.Is(err, ErrNoProfile) {
		t.Errorf("error = %v, want ErrNoProfile", err)
	}
}

func TestResolve_FlagProfileNotFound(t *testing.T) {
	store := &ProfileStore{
		Profiles: map[string]*Profile{
			"default": {Username: "user"},
		},
	}

	_, err := resolveFromStore(store, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent flag profile")
	}
	if got := err.Error(); !contains(got, "not found") || !contains(got, "--profile flag") {
		t.Errorf("error message = %q, should mention 'not found' and '--profile flag'", got)
	}
}

func TestResolve_EnvProfileNotFound(t *testing.T) {
	t.Setenv("CTX_PROFILE", "nonexistent")

	store := &ProfileStore{
		Profiles: map[string]*Profile{
			"default": {Username: "user"},
		},
	}

	_, err := resolveFromStore(store, "")
	if err == nil {
		t.Fatal("expected error for nonexistent env profile")
	}
	if got := err.Error(); !contains(got, "not found") || !contains(got, "CTX_PROFILE") {
		t.Errorf("error message = %q, should mention 'not found' and 'CTX_PROFILE'", got)
	}
}

func TestFindProjectProfile_WalkUp(t *testing.T) {
	tmp := t.TempDir()
	// Create .ctx-profile in parent
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("parent-profile"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create child dir
	child := filepath.Join(tmp, "child", "grandchild")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	name := findProjectProfileFrom(child)
	if name != "parent-profile" {
		t.Errorf("FindProjectProfile = %q, want %q", name, "parent-profile")
	}
}

func TestFindProjectProfile_NestedOverride(t *testing.T) {
	tmp := t.TempDir()
	// Parent .ctx-profile
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("parent"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Child .ctx-profile
	child := filepath.Join(tmp, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, ".ctx-profile"), []byte("child"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	name := findProjectProfileFrom(child)
	if name != "child" {
		t.Errorf("FindProjectProfile = %q, want %q (nearest wins)", name, "child")
	}
}

func TestFindProjectProfile_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	name := findProjectProfileFrom(tmp)
	if name != "" {
		t.Errorf("FindProjectProfile = %q, want empty for empty file", name)
	}
}

func TestFindProjectProfile_WhitespaceOnly(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("  \t  "), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	name := findProjectProfileFrom(tmp)
	if name != "" {
		t.Errorf("FindProjectProfile = %q, want empty for whitespace-only file", name)
	}
}

func TestFindProjectProfile_MultiLine(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("work\nextra"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	name := findProjectProfileFrom(tmp)
	if name != "" {
		t.Errorf("FindProjectProfile = %q, want empty for multi-line file", name)
	}
}

func TestFindProjectProfile_TrailingNewline(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".ctx-profile"), []byte("work\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	name := findProjectProfileFrom(tmp)
	if name != "work" {
		t.Errorf("FindProjectProfile = %q, want %q (trailing newline should be trimmed)", name, "work")
	}
}

func TestFindProjectProfile_NoFile(t *testing.T) {
	tmp := t.TempDir()
	name := findProjectProfileFrom(tmp)
	if name != "" {
		t.Errorf("FindProjectProfile = %q, want empty when no file", name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
