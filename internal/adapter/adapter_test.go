package adapter

import (
	"runtime"
	"testing"
)

func TestPlatform(t *testing.T) {
	p := Platform()
	if p == "" {
		t.Error("Platform() returned empty string")
	}
	expected := runtime.GOOS + "-" + runtime.GOARCH
	if p != expected {
		t.Errorf("Platform() = %q, want %q", p, expected)
	}
}

func TestDetectAdapters(t *testing.T) {
	adapters := DetectAdapters()
	// Binary adapter should always be available
	found := false
	for _, a := range adapters {
		if a.Name() == "binary" {
			found = true
			break
		}
	}
	if !found {
		t.Error("BinaryAdapter should always be available")
	}
}

func TestFindAdapter(t *testing.T) {
	tests := []struct {
		name    string
		spec    InstallSpec
		wantErr bool
	}{
		{
			name: "binary direct URL",
			spec: InstallSpec{
				Source: "https://example.com/tool.tar.gz",
			},
			wantErr: false,
		},
		{
			name: "platform-specific binary",
			spec: InstallSpec{
				Platforms: map[string]PlatformSpec{
					Platform(): {Binary: "https://example.com/tool.tar.gz"},
				},
			},
			wantErr: false,
		},
		{
			name:    "no adapter available",
			spec:    InstallSpec{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, pkg, err := FindAdapter(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if adapter == nil {
				t.Error("adapter is nil")
			}
			if pkg == "" {
				t.Error("package string is empty")
			}
		})
	}
}

func TestVerify(t *testing.T) {
	// "go" should be available in test environment
	if err := Verify("go", "go version"); err != nil {
		t.Errorf("Verify(go) failed: %v", err)
	}

	// nonexistent binary should fail
	if err := Verify("nonexistent-binary-xyz", ""); err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestCommandExists(t *testing.T) {
	if !commandExists("go") {
		t.Error("go should exist")
	}
	if commandExists("nonexistent-cmd-xyz") {
		t.Error("nonexistent-cmd-xyz should not exist")
	}
}
