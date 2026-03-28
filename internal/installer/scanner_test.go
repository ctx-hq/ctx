package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestScanInstalled(t *testing.T) {
	dataDir := t.TempDir()
	inst := &Installer{DataDir: dataDir}

	// Create two packages
	packages := []struct {
		fullName    string
		version     string
		pkgType     manifest.PackageType
		description string
	}{
		{"@test/alpha", "1.0.0", manifest.TypeSkill, "Alpha skill"},
		{"@test/beta", "2.0.0", manifest.TypeMCP, "Beta MCP"},
	}

	for _, pkg := range packages {
		vDir := filepath.Join(dataDir, pkg.fullName, pkg.version)
		os.MkdirAll(vDir, 0o755)
		m := manifest.Manifest{
			Name:        pkg.fullName,
			Version:     pkg.version,
			Type:        pkg.pkgType,
			Description: pkg.description,
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		os.WriteFile(filepath.Join(vDir, "manifest.json"), data, 0o644)
		SwitchCurrent(filepath.Join(dataDir, pkg.fullName), pkg.version)
	}

	result, err := inst.ScanInstalled()
	if err != nil {
		t.Fatalf("ScanInstalled() error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("ScanInstalled() = %d packages, want 2", len(result))
	}

	// Results should be sorted by full_name
	if result[0].FullName != "@test/alpha" {
		t.Errorf("first package = %q, want @test/alpha", result[0].FullName)
	}
	if result[1].FullName != "@test/beta" {
		t.Errorf("second package = %q, want @test/beta", result[1].FullName)
	}

	// Check fields
	if result[0].Version != "1.0.0" {
		t.Errorf("alpha version = %q, want 1.0.0", result[0].Version)
	}
	if result[0].Type != "skill" {
		t.Errorf("alpha type = %q, want skill", result[0].Type)
	}
	if result[0].Description != "Alpha skill" {
		t.Errorf("alpha description = %q, want 'Alpha skill'", result[0].Description)
	}
	if result[1].Type != "mcp" {
		t.Errorf("beta type = %q, want mcp", result[1].Type)
	}
}

func TestScanInstalled_Empty(t *testing.T) {
	dataDir := t.TempDir()
	inst := &Installer{DataDir: dataDir}

	result, err := inst.ScanInstalled()
	if err != nil {
		t.Fatalf("ScanInstalled() error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ScanInstalled() = %d packages, want 0", len(result))
	}
}

func TestScanInstalled_NonExistentDir(t *testing.T) {
	inst := &Installer{DataDir: "/nonexistent/path"}

	result, err := inst.ScanInstalled()
	if err != nil {
		t.Fatalf("ScanInstalled() error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ScanInstalled() = %d packages, want 0", len(result))
	}
}

func TestScanInstalled_CorruptManifest(t *testing.T) {
	dataDir := t.TempDir()
	inst := &Installer{DataDir: dataDir}

	// Create a package with corrupt manifest
	vDir := filepath.Join(dataDir, "@test", "corrupt", "1.0.0")
	os.MkdirAll(vDir, 0o755)
	os.WriteFile(filepath.Join(vDir, "manifest.json"), []byte("not json"), 0o644)
	pkgDir := filepath.Join(dataDir, "@test", "corrupt")
	SwitchCurrent(pkgDir, "1.0.0")

	// Create a valid package
	vDir2 := filepath.Join(dataDir, "@test", "valid", "1.0.0")
	os.MkdirAll(vDir2, 0o755)
	m := manifest.Manifest{Name: "@test/valid", Version: "1.0.0", Type: manifest.TypeSkill}
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(vDir2, "manifest.json"), data, 0o644)
	SwitchCurrent(filepath.Join(dataDir, "@test", "valid"), "1.0.0")

	result, err := inst.ScanInstalled()
	if err != nil {
		t.Fatalf("ScanInstalled() error: %v", err)
	}

	// Should skip corrupt and return only valid
	if len(result) != 1 {
		t.Fatalf("ScanInstalled() = %d packages, want 1", len(result))
	}
	if result[0].FullName != "@test/valid" {
		t.Errorf("package = %q, want @test/valid", result[0].FullName)
	}
}

func TestIsInstalled(t *testing.T) {
	dataDir := t.TempDir()
	inst := &Installer{DataDir: dataDir}

	// Not installed
	if inst.IsInstalled("@test/missing") {
		t.Error("IsInstalled() = true for non-existent package")
	}

	// Install a package
	vDir := filepath.Join(dataDir, "@test", "present", "1.0.0")
	os.MkdirAll(vDir, 0o755)
	m := manifest.Manifest{Name: "@test/present", Version: "1.0.0", Type: manifest.TypeSkill}
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(vDir, "manifest.json"), data, 0o644)
	SwitchCurrent(filepath.Join(dataDir, "@test", "present"), "1.0.0")

	if !inst.IsInstalled("@test/present") {
		t.Error("IsInstalled() = false for installed package")
	}
}

func TestGetInstalled(t *testing.T) {
	dataDir := t.TempDir()
	inst := &Installer{DataDir: dataDir}

	// Non-existent package
	_, err := inst.GetInstalled("@test/missing")
	if err == nil {
		t.Error("GetInstalled() should error for missing package")
	}

	// Install a package
	vDir := filepath.Join(dataDir, "@test", "myskill", "2.0.0")
	os.MkdirAll(vDir, 0o755)
	m := manifest.Manifest{
		Name:        "@test/myskill",
		Version:     "2.0.0",
		Type:        manifest.TypeSkill,
		Description: "My test skill",
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(vDir, "manifest.json"), data, 0o644)
	SwitchCurrent(filepath.Join(dataDir, "@test", "myskill"), "2.0.0")

	pkg, err := inst.GetInstalled("@test/myskill")
	if err != nil {
		t.Fatalf("GetInstalled() error: %v", err)
	}
	if pkg.FullName != "@test/myskill" {
		t.Errorf("FullName = %q, want @test/myskill", pkg.FullName)
	}
	if pkg.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", pkg.Version)
	}
	if pkg.Type != "skill" {
		t.Errorf("Type = %q, want skill", pkg.Type)
	}
	if pkg.Description != "My test skill" {
		t.Errorf("Description = %q, want 'My test skill'", pkg.Description)
	}
}
