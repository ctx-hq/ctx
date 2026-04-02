package installer

import (
	"context"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestRunPostInstallHooks_NilMCP(t *testing.T) {
	m := &manifest.Manifest{}
	completed, err := RunPostInstallHooks(context.Background(), m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if completed != nil {
		t.Errorf("expected nil completed, got %v", completed)
	}
}

func TestRunPostInstallHooks_NoHooks(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{Transport: "stdio", Command: "node"},
	}
	completed, err := RunPostInstallHooks(context.Background(), m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if completed != nil {
		t.Errorf("expected nil completed, got %v", completed)
	}
}

func TestRunPostInstallHooks_EmptyHooks(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks:     &manifest.MCPHooks{},
		},
	}
	completed, err := RunPostInstallHooks(context.Background(), m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if completed != nil {
		t.Errorf("expected nil completed, got %v", completed)
	}
}

func TestRunPostInstallHooks_Success(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "echo", Args: []string{"hello"}, Description: "Say hello"},
				},
			},
		},
	}
	completed, err := RunPostInstallHooks(context.Background(), m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(completed) != 1 || completed[0] != "Say hello" {
		t.Errorf("expected completed=[Say hello], got %v", completed)
	}
}

func TestRunPostInstallHooks_RequiredFailure(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "false", Description: "Always fails", Optional: false},
				},
			},
		},
	}
	_, err := RunPostInstallHooks(context.Background(), m, nil)
	if err == nil {
		t.Fatal("expected error for required hook failure")
	}
}

func TestRunPostInstallHooks_OptionalFailure(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "false", Description: "Optional fail", Optional: true},
					{Command: "echo", Args: []string{"ok"}, Description: "After optional"},
				},
			},
		},
	}
	completed, err := RunPostInstallHooks(context.Background(), m, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Optional hook fails but continues; second hook succeeds
	if len(completed) != 1 || completed[0] != "After optional" {
		t.Errorf("expected completed=[After optional], got %v", completed)
	}
}

func TestRunPostInstallHooks_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "sleep", Args: []string{"10"}, Description: "Long hook"},
				},
			},
		},
	}
	_, err := RunPostInstallHooks(ctx, m, nil)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestRunPostInstallHooks_ConfirmDenied(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "echo", Args: []string{"should not run"}, Description: "Denied hook"},
				},
			},
		},
	}
	deny := func() (bool, error) { return false, nil }
	completed, err := RunPostInstallHooks(context.Background(), m, deny)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(completed) != 0 {
		t.Errorf("expected no completed hooks when denied, got %v", completed)
	}
}

func TestRunPostInstallHooks_ConfirmAccepted(t *testing.T) {
	m := &manifest.Manifest{
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Hooks: &manifest.MCPHooks{
				PostInstall: []manifest.HookStep{
					{Command: "echo", Args: []string{"yes"}, Description: "Accepted hook"},
				},
			},
		},
	}
	accept := func() (bool, error) { return true, nil }
	completed, err := RunPostInstallHooks(context.Background(), m, accept)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(completed) != 1 {
		t.Errorf("expected 1 completed hook, got %v", completed)
	}
}
