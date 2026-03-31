package doctor

import (
	"testing"
)

func TestRunChecks_ReturnsChecks(t *testing.T) {
	result := RunChecks("test-version", "")

	if len(result.Checks) == 0 {
		t.Fatal("expected at least one check")
	}

	// First check should always be version
	if result.Checks[0].Name != "ctx version" {
		t.Errorf("first check: got %q, want %q", result.Checks[0].Name, "ctx version")
	}
	if result.Checks[0].Status != "pass" {
		t.Errorf("version check status: got %q, want %q", result.Checks[0].Status, "pass")
	}
	if result.Checks[0].Detail == "" {
		t.Error("version check detail should not be empty")
	}

	// Auth should be warn when no token
	for _, c := range result.Checks {
		if c.Name == "auth" {
			if c.Status != "warn" {
				t.Errorf("auth with empty token: got %q, want %q", c.Status, "warn")
			}
			break
		}
	}
}

func TestRunChecks_CountsCorrect(t *testing.T) {
	result := RunChecks("test-version", "")

	total := result.PassCount + result.WarnCount + result.FailCount
	if total != len(result.Checks) {
		t.Errorf("count mismatch: pass(%d)+warn(%d)+fail(%d)=%d, but len(checks)=%d",
			result.PassCount, result.WarnCount, result.FailCount, total, len(result.Checks))
	}
}

func TestResult_Summary(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name: "all pass",
			result: Result{
				Checks:    make([]Check, 3),
				PassCount: 3,
			},
			want: "3 checks: 3 pass",
		},
		{
			name: "mixed",
			result: Result{
				Checks:    make([]Check, 5),
				PassCount: 3,
				WarnCount: 1,
				FailCount: 1,
			},
			want: "5 checks: 3 pass, 1 warn, 1 fail",
		},
		{
			name: "warn only appended when nonzero",
			result: Result{
				Checks:    make([]Check, 2),
				PassCount: 1,
				FailCount: 1,
			},
			want: "2 checks: 1 pass, 1 fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Summary()
			if got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}
