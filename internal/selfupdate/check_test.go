package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"0.4.0", "0.3.0", true},
		{"0.3.0", "0.3.0", false},
		{"0.3.0", "0.4.0", false},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"2.0.0", "1.99.99", true},
		{"0.3.1", "0.3.0", true},
		{"0.3.0", "0.3.1", false},
		{"", "0.3.0", false},
		{"0.3.0", "", false},
		{"dev", "0.3.0", false},
	}
	for _, tt := range tests {
		got := IsNewer(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestIsUpToDate(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"0.4.0", "0.4.0", true},   // same version
		{"0.3.0", "0.4.0", true},   // current is newer
		{"0.4.0", "0.3.0", false},  // current is older
		{"0.4.0", "dev", false},    // dev → not up to date, should upgrade
		{"0.4.0", "", false},       // empty current → not up to date
		{"", "0.4.0", false},       // empty latest → not up to date
		{"0.4.0", "0.4.0-beta", true}, // prerelease suffix is stripped by parseSemver, so 0.4.0 == 0.4.0
		{"1.0.0", "0.9.9", false},  // current is older
		{"0.3.0", "1.0.0", true},   // current is newer
	}
	for _, tt := range tests {
		got := IsUpToDate(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("IsUpToDate(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"0.3.0", []int{0, 3, 0}},
		{"v1.2.3", []int{1, 2, 3}},
		{"1.0.0-beta", []int{1, 0, 0}},
		{"10.20.30", []int{10, 20, 30}},
		{"invalid", nil},
		{"", nil},
		{"1.2", nil},
		{"a.b.c", nil},
	}
	for _, tt := range tests {
		got := parseSemver(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("parseSemver(%q) = %v, want nil", tt.input, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("parseSemver(%q) = nil, want %v", tt.input, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("parseSemver(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestLoadSaveCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	now := time.Now().UTC().Truncate(time.Second)
	original := &UpdateCache{
		LastCheck:     now,
		LatestVersion: "1.2.3",
	}

	saveCache(path, original)

	loaded, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if loaded.LatestVersion != original.LatestVersion {
		t.Errorf("LatestVersion = %q, want %q", loaded.LatestVersion, original.LatestVersion)
	}
	// Time comparison with tolerance for JSON round-trip
	if loaded.LastCheck.Sub(original.LastCheck).Abs() > time.Second {
		t.Errorf("LastCheck = %v, want ~%v", loaded.LastCheck, original.LastCheck)
	}
}

func TestLoadCache_Missing(t *testing.T) {
	_, err := loadCache(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestLoadCache_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadCache(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCheckForUpdate_DevVersion(t *testing.T) {
	if got := CheckForUpdate("dev"); got != "" {
		t.Errorf("CheckForUpdate(dev) = %q, want empty", got)
	}
	if got := CheckForUpdate(""); got != "" {
		t.Errorf("CheckForUpdate('') = %q, want empty", got)
	}
}

func TestCheckForUpdate_CachedUpToDate(t *testing.T) {
	// Set up a cache file that says "0.3.0" is latest, checked recently
	dir := t.TempDir()
	t.Setenv("CTX_HOME", dir)

	cache := &UpdateCache{
		LastCheck:     time.Now().UTC(),
		LatestVersion: "0.3.0",
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(dir, cacheFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Current version is same as cached latest — should return ""
	if got := CheckForUpdate("0.3.0"); got != "" {
		t.Errorf("CheckForUpdate(0.3.0) with cache 0.3.0 = %q, want empty", got)
	}
}

func TestCheckForUpdate_CachedNewer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CTX_HOME", dir)

	cache := &UpdateCache{
		LastCheck:     time.Now().UTC(),
		LatestVersion: "0.5.0",
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(dir, cacheFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Current version is older than cached latest
	if got := CheckForUpdate("0.3.0"); got != "0.5.0" {
		t.Errorf("CheckForUpdate(0.3.0) with cache 0.5.0 = %q, want 0.5.0", got)
	}
}
