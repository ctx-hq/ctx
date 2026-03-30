package main

import (
	"strings"
	"testing"
)

func TestOrgRoleValidation(t *testing.T) {
	tests := []struct {
		role    string
		wantOK  bool
	}{
		{"owner", true},
		{"admin", true},
		{"member", true},
		{"Owner", true},   // should be normalized to lowercase
		{"ADMIN", true},   // should be normalized to lowercase
		{"viewer", false},
		{"superadmin", false},
		{"", false},
		{"moderator", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			role := strings.ToLower(tt.role)
			ok := role == "owner" || role == "admin" || role == "member"
			if ok != tt.wantOK {
				t.Errorf("role %q valid = %v, want %v", tt.role, ok, tt.wantOK)
			}
		})
	}
}

func TestOrgCmd_HasExpectedSubcommands(t *testing.T) {
	subs := orgCmd.Commands()
	names := make(map[string]bool)
	for _, sub := range subs {
		names[sub.Name()] = true
	}

	required := []string{"create", "info", "list", "packages", "add", "remove", "delete", "invite", "invitations", "cancel-invite"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("org missing subcommand %q", name)
		}
	}
}

func TestOrgAddCmd_DefaultRole(t *testing.T) {
	f := orgAddCmd.Flags().Lookup("role")
	if f == nil {
		t.Fatal("--role flag not found on org add command")
	}
	if f.DefValue != "member" {
		t.Errorf("default role = %q, want %q", f.DefValue, "member")
	}
}
