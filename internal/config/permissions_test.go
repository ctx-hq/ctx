package config

import (
	"runtime"
	"strings"
	"testing"
)

func TestUserAgent_Format(t *testing.T) {
	Version = "1.2.3"
	ua := UserAgent()

	if !strings.HasPrefix(ua, "ctx/1.2.3") {
		t.Errorf("UserAgent() = %q, should start with ctx/1.2.3", ua)
	}
	expected := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(ua, expected) {
		t.Errorf("UserAgent() = %q, should contain %q", ua, expected)
	}
}

func TestUserAgent_DevDefault(t *testing.T) {
	Version = ""
	ua := UserAgent()
	if !strings.HasPrefix(ua, "ctx/dev") {
		t.Errorf("UserAgent() = %q, should start with ctx/dev when version is empty", ua)
	}
}

func TestPermissionConstants(t *testing.T) {
	if DirPerm != 0o700 {
		t.Errorf("DirPerm = %o, want 0700", DirPerm)
	}
	if FilePerm != 0o600 {
		t.Errorf("FilePerm = %o, want 0600", FilePerm)
	}
	if BinPerm != 0o755 {
		t.Errorf("BinPerm = %o, want 0755", BinPerm)
	}
}
