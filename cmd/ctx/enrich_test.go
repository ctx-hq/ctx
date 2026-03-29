package main

import (
	"testing"
)

func TestEnrichFlags_ResetIsBoolean(t *testing.T) {
	cmd := enrichCmd

	f := cmd.Flags().Lookup("reset")
	if f == nil {
		t.Fatal("--reset flag not registered on enrich command")
	}
	if f.DefValue != "false" {
		t.Errorf("--reset default = %q, want %q", f.DefValue, "false")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("--reset type = %q, want %q", f.Value.Type(), "bool")
	}
}

func TestEnrichFlags_RedoRemoved(t *testing.T) {
	cmd := enrichCmd
	if f := cmd.Flags().Lookup("redo"); f != nil {
		t.Error("--redo flag should have been removed")
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
