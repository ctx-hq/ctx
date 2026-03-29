package auth

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestOpenBrowser_CommandConstruction(t *testing.T) {
	// Capture the command that OpenBrowser would execute
	var gotName string
	var gotArgs []string

	origExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		// Return a harmless command so .Start() succeeds
		return exec.Command("true")
	}
	defer func() { execCommand = origExecCommand }()

	testURL := "https://example.com/auth?code=ABC&state=xyz"
	err := OpenBrowser(testURL)
	if err != nil {
		t.Fatalf("OpenBrowser returned error: %v", err)
	}

	switch runtime.GOOS {
	case "darwin":
		if gotName != "open" {
			t.Errorf("command = %q, want %q", gotName, "open")
		}
		if len(gotArgs) != 1 || gotArgs[0] != testURL {
			t.Errorf("args = %v, want [%q]", gotArgs, testURL)
		}
	case "linux":
		if gotName != "xdg-open" {
			t.Errorf("command = %q, want %q", gotName, "xdg-open")
		}
		if len(gotArgs) != 1 || gotArgs[0] != testURL {
			t.Errorf("args = %v, want [%q]", gotArgs, testURL)
		}
	case "windows":
		if gotName != "cmd" {
			t.Errorf("command = %q, want %q", gotName, "cmd")
		}
		// Verify the empty title argument is present
		expected := []string{"/c", "start", "", testURL}
		if len(gotArgs) != len(expected) {
			t.Fatalf("args = %v, want %v", gotArgs, expected)
		}
		for i, v := range expected {
			if gotArgs[i] != v {
				t.Errorf("args[%d] = %q, want %q", i, gotArgs[i], v)
			}
		}
	default:
		t.Skipf("unsupported platform %s", runtime.GOOS)
	}
}

func TestOpenBrowser_UnsupportedPlatform(t *testing.T) {
	// This test only runs on unsupported platforms (unlikely in CI).
	// On supported platforms, we verify via TestOpenBrowser_CommandConstruction.
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		t.Skip("skipping on supported platform")
	}

	err := OpenBrowser("https://example.com")
	if err == nil {
		t.Error("expected error on unsupported platform")
	}
}
