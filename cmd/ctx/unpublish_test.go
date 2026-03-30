package main

import "testing"

func TestParseUnpublishRef_WholePackage(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantVer  string
		wantDel  bool
	}{
		{"@hong/my-skill", "@hong/my-skill", "", false},
		{"@biao29/fizzy-cli", "@biao29/fizzy-cli", "", false},
	}
	for _, tt := range tests {
		name, ver, isDel, err := parseUnpublishRef(tt.ref)
		if err != nil {
			t.Errorf("parseUnpublishRef(%q) unexpected error: %v", tt.ref, err)
			continue
		}
		if name != tt.wantName || ver != tt.wantVer || isDel != tt.wantDel {
			t.Errorf("parseUnpublishRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.ref, name, ver, isDel, tt.wantName, tt.wantVer, tt.wantDel)
		}
	}
}

func TestParseUnpublishRef_WithVersion(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantVer  string
		wantDel  bool
	}{
		{"@hong/my-skill@1.0.0", "@hong/my-skill", "1.0.0", true},
		{"@biao29/fizzy-cli@0.1.0", "@biao29/fizzy-cli", "0.1.0", true},
		{"@org/pkg@2.0.0-beta.1", "@org/pkg", "2.0.0-beta.1", true},
	}
	for _, tt := range tests {
		name, ver, isDel, err := parseUnpublishRef(tt.ref)
		if err != nil {
			t.Errorf("parseUnpublishRef(%q) unexpected error: %v", tt.ref, err)
			continue
		}
		if name != tt.wantName || ver != tt.wantVer || isDel != tt.wantDel {
			t.Errorf("parseUnpublishRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.ref, name, ver, isDel, tt.wantName, tt.wantVer, tt.wantDel)
		}
	}
}

func TestParseUnpublishRef_InvalidInputs(t *testing.T) {
	tests := []struct {
		ref  string
		desc string
	}{
		{"bare-name", "no @ prefix"},
		{"@hong/my-skill@", "trailing @ with empty version"},
		{"@/name@1.0.0", "empty scope"},
		{"@scope/@1.0.0", "empty name"},
		{"@noslash", "missing slash in whole-package ref"},
		{"@noslash@1.0.0", "missing slash in versioned ref"},
		{"@/name", "empty scope in whole-package ref"},
		{"@scope/", "empty name in whole-package ref"},
	}
	for _, tt := range tests {
		_, _, _, err := parseUnpublishRef(tt.ref)
		if err == nil {
			t.Errorf("parseUnpublishRef(%q) [%s]: expected error, got nil", tt.ref, tt.desc)
		}
	}
}

func TestUnpublishCmd_AcceptsOneArg(t *testing.T) {
	if err := unpublishCmd.Args(unpublishCmd, nil); err == nil {
		t.Error("should reject 0 args")
	}
	if err := unpublishCmd.Args(unpublishCmd, []string{"@hong/pkg"}); err != nil {
		t.Errorf("should accept 1 arg: %v", err)
	}
	if err := unpublishCmd.Args(unpublishCmd, []string{"a", "b"}); err == nil {
		t.Error("should reject 2 args")
	}
}
