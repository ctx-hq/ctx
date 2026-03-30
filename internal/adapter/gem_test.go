package adapter

import "testing"

func TestGemAdapterName(t *testing.T) {
	a := &GemAdapter{}
	if a.Name() != "gem" {
		t.Errorf("Name() = %q, want %q", a.Name(), "gem")
	}
}

// Compile-time interface check.
var _ Adapter = (*GemAdapter)(nil)

func TestFindByNameGem(t *testing.T) {
	a, err := FindByName("gem")
	if err != nil {
		t.Fatalf("FindByName(gem): %v", err)
	}
	if a.Name() != "gem" {
		t.Errorf("Name() = %q, want %q", a.Name(), "gem")
	}
}

func TestFindByNameUnknown(t *testing.T) {
	_, err := FindByName("nonexistent")
	if err == nil {
		t.Error("FindByName(nonexistent) should error")
	}
}

func TestFindAdapterGem(t *testing.T) {
	spec := InstallSpec{Gem: "fizzy-cli"}
	a, pkg, err := FindAdapter(spec)
	if err != nil {
		// gem might not be installed; skip if so
		if !commandExists("gem") {
			t.Skip("gem not installed")
		}
		t.Fatalf("FindAdapter: %v", err)
	}
	if a.Name() != "gem" {
		t.Errorf("adapter = %q, want gem", a.Name())
	}
	if pkg != "fizzy-cli" {
		t.Errorf("pkg = %q, want fizzy-cli", pkg)
	}
}
