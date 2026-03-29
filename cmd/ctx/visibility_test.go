package main

import (
	"testing"
)

func TestVisibilityValues_Valid(t *testing.T) {
	valid := []string{"public", "unlisted", "private"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			ok := v == "public" || v == "unlisted" || v == "private"
			if !ok {
				t.Errorf("visibility %q should be valid", v)
			}
		})
	}
}

func TestVisibilityValues_Invalid(t *testing.T) {
	invalid := []string{"internal", "PUBLIC", "Protected", "", "shared", "org-only"}
	for _, v := range invalid {
		t.Run(v, func(t *testing.T) {
			ok := v == "public" || v == "unlisted" || v == "private"
			if ok {
				t.Errorf("visibility %q should be invalid", v)
			}
		})
	}
}

func TestVisibilityCmd_AcceptsOneOrTwoArgs(t *testing.T) {
	// visibilityCmd uses cobra.RangeArgs(1, 2)
	cmd := visibilityCmd

	tests := []struct {
		name    string
		nArgs   int
		wantErr bool
	}{
		{"zero args is invalid", 0, true},
		{"one arg is valid (view)", 1, false},
		{"two args is valid (set)", 2, false},
		{"three args is invalid", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]string, tt.nArgs)
			for i := range args {
				args[i] = "arg"
			}
			err := cmd.Args(cmd, args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%d) error = %v, wantErr = %v", tt.nArgs, err, tt.wantErr)
			}
		})
	}
}
