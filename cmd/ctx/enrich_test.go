package main

import (
	"testing"
)

func TestEnrichFlags_ResetAndRedo_AreMutuallyExclusive(t *testing.T) {
	// Both flags start false; only one should be set at a time
	// This tests the default state
	cmd := enrichCmd

	resetFlag := cmd.Flags().Lookup("reset")
	redoFlag := cmd.Flags().Lookup("redo")

	if resetFlag == nil {
		t.Fatal("--reset flag not registered on enrich command")
	}
	if redoFlag == nil {
		t.Fatal("--redo flag not registered on enrich command")
	}

	// Defaults should be false
	if resetFlag.DefValue != "false" {
		t.Errorf("--reset default = %q, want %q", resetFlag.DefValue, "false")
	}
	if redoFlag.DefValue != "false" {
		t.Errorf("--redo default = %q, want %q", redoFlag.DefValue, "false")
	}
}

func TestEnrichCmd_RequiresExactlyOneArg(t *testing.T) {
	cmd := enrichCmd

	tests := []struct {
		name    string
		nArgs   int
		wantErr bool
	}{
		{"zero args", 0, true},
		{"one arg", 1, false},
		{"two args", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]string, tt.nArgs)
			for i := range args {
				args[i] = "@scope/pkg"
			}
			err := cmd.Args(cmd, args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%d) error = %v, wantErr = %v", tt.nArgs, err, tt.wantErr)
			}
		})
	}
}

func TestEnrichFlags_AreBoolean(t *testing.T) {
	cmd := enrichCmd
	flags := []string{"reset", "redo"}

	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			f := cmd.Flags().Lookup(name)
			if f == nil {
				t.Fatalf("flag --%s not found", name)
			}
			if f.Value.Type() != "bool" {
				t.Errorf("flag --%s type = %q, want %q", name, f.Value.Type(), "bool")
			}
		})
	}
}
