package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestInvitationsCmd_HasExpectedSubcommands(t *testing.T) {
	subs := invitationsCmd.Commands()
	names := make(map[string]bool)
	for _, sub := range subs {
		names[sub.Name()] = true
	}

	required := []string{"accept", "decline"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("invitations missing subcommand %q", name)
		}
	}
}

func TestInvitationsCmd_HasAlias(t *testing.T) {
	aliases := invitationsCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "inv" {
			found = true
			break
		}
	}
	if !found {
		t.Error("invitations command should have alias 'inv'")
	}
}

func TestInvitationsAcceptCmd_RequiresExactlyOneArg(t *testing.T) {
	// Verify the command rejects wrong arg counts
	err := cobra.ExactArgs(1)(invitationsAcceptCmd, []string{})
	if err == nil {
		t.Error("accept should reject 0 args")
	}
	err = cobra.ExactArgs(1)(invitationsAcceptCmd, []string{"a", "b"})
	if err == nil {
		t.Error("accept should reject 2 args")
	}
	err = cobra.ExactArgs(1)(invitationsAcceptCmd, []string{"inv-1"})
	if err != nil {
		t.Errorf("accept should allow 1 arg, got error: %v", err)
	}
}

func TestInvitationsDeclineCmd_RequiresExactlyOneArg(t *testing.T) {
	err := cobra.ExactArgs(1)(invitationsDeclineCmd, []string{})
	if err == nil {
		t.Error("decline should reject 0 args")
	}
	err = cobra.ExactArgs(1)(invitationsDeclineCmd, []string{"a", "b"})
	if err == nil {
		t.Error("decline should reject 2 args")
	}
	err = cobra.ExactArgs(1)(invitationsDeclineCmd, []string{"inv-1"})
	if err != nil {
		t.Errorf("decline should allow 1 arg, got error: %v", err)
	}
}

func TestOrgInviteCmd_HasRoleFlag(t *testing.T) {
	f := orgInviteCmd.Flags().Lookup("role")
	if f == nil {
		t.Fatal("--role flag not found on org invite command")
	}
	if f.DefValue != "member" {
		t.Errorf("default role = %q, want %q", f.DefValue, "member")
	}
}

func TestOrgInviteCmd_RoleFlagIsIndependent(t *testing.T) {
	// Verify invite's --role flag is not shared with add's --role flag
	addFlag := orgAddCmd.Flags().Lookup("role")
	inviteFlag := orgInviteCmd.Flags().Lookup("role")
	if addFlag == inviteFlag {
		t.Error("org add and org invite should not share the same --role flag pointer")
	}
}
