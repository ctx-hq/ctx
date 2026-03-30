package main

import (
	"testing"
)

func TestAccessCmd_HasExpectedSubcommands(t *testing.T) {
	subs := accessCmd.Commands()
	names := make(map[string]bool)
	for _, sub := range subs {
		names[sub.Name()] = true
	}

	required := []string{"grant", "revoke"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("access missing subcommand %q", name)
		}
	}
}

func TestAccessGrantCmd_RequiresMinArgs(t *testing.T) {
	if accessGrantCmd.Args == nil {
		t.Error("grant command should require minimum 2 args")
	}
}

func TestAccessRevokeCmd_RequiresMinArgs(t *testing.T) {
	if accessRevokeCmd.Args == nil {
		t.Error("revoke command should require minimum 2 args")
	}
}
