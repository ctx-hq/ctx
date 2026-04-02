package installer

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestBuildMCPConfig_DefaultTransport(t *testing.T) {
	mcp := &manifest.MCPSpec{
		Transport: "stdio",
		Command:   "docker",
		Args:      []string{"run", "-i", "image"},
		URL:       "",
	}
	cfg, err := buildMCPConfig(mcp, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Command != "docker" {
		t.Errorf("Command = %q, want docker", cfg.Command)
	}
	if len(cfg.Args) != 3 {
		t.Errorf("Args = %v, want 3 items", cfg.Args)
	}
}

func TestBuildMCPConfig_SelectTransport(t *testing.T) {
	mcp := &manifest.MCPSpec{
		Transport: "stdio",
		Command:   "docker",
		Args:      []string{"run", "image"},
		Transports: []manifest.TransportSpec{
			{
				ID:        "remote",
				Transport: "streamable-http",
				URL:       "https://api.example.com/mcp/",
			},
			{
				ID:        "stdio-docker",
				Transport: "stdio",
				Command:   "docker",
				Args:      []string{"run", "-i", "--rm", "other-image"},
			},
		},
	}

	// Select remote transport
	cfg, err := buildMCPConfig(mcp, "remote")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "https://api.example.com/mcp/" {
		t.Errorf("URL = %q, want https://api.example.com/mcp/", cfg.URL)
	}
	if cfg.Command != "" {
		t.Errorf("Command = %q, want empty for remote", cfg.Command)
	}

	// Select stdio-docker transport
	cfg, err = buildMCPConfig(mcp, "stdio-docker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Command != "docker" {
		t.Errorf("Command = %q, want docker", cfg.Command)
	}
	if len(cfg.Args) != 4 {
		t.Errorf("Args = %v, want 4 items", cfg.Args)
	}

	// Unknown transport ID returns error
	_, err = buildMCPConfig(mcp, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown transport ID, got nil")
	}
}

func TestBuildMCPConfig_EmptyTransports(t *testing.T) {
	mcp := &manifest.MCPSpec{
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "@playwright/mcp"},
	}
	// With transportID but no transports[] defined, should return error
	_, err := buildMCPConfig(mcp, "anything")
	if err == nil {
		t.Error("expected error for transport ID with no transports defined, got nil")
	}
}
