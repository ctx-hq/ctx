package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAll(t *testing.T) {
	agents := DetectAll()
	// At minimum, generic agent should always be detected
	found := false
	for _, a := range agents {
		if a.Name() == "generic" {
			found = true
		}
	}
	if !found {
		t.Error("generic agent should always be detected")
	}
}

func TestFindByName(t *testing.T) {
	a, err := FindByName("claude")
	if err != nil {
		t.Fatalf("FindByName(claude) error: %v", err)
	}
	if a.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", a.Name(), "claude")
	}

	_, err = FindByName("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestWriteAndRemoveMCPConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	// Write MCP config
	cfg := MCPConfig{
		Command: "npx",
		Args:    []string{"-y", "@mcp/github"},
		Env:     map[string]string{"GITHUB_TOKEN": "test"},
	}
	err := writeMCPConfig(configPath, "github-mcp", cfg)
	if err != nil {
		t.Fatalf("writeMCPConfig error: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	servers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found")
	}
	entry, ok := servers["github-mcp"].(map[string]any)
	if !ok {
		t.Fatal("github-mcp entry not found")
	}
	if entry["command"] != "npx" {
		t.Errorf("command = %v, want npx", entry["command"])
	}

	// Add another MCP
	cfg2 := MCPConfig{Command: "node", Args: []string{"server.js"}}
	if err := writeMCPConfig(configPath, "other-mcp", cfg2); err != nil {
		t.Fatalf("writeMCPConfig (other-mcp): %v", err)
	}

	data2, _ := os.ReadFile(configPath)
	var result2 map[string]any
	if err := json.Unmarshal(data2, &result2); err != nil {
		t.Fatalf("parse config2: %v", err)
	}
	servers2 := result2["mcpServers"].(map[string]any)
	if len(servers2) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers2))
	}

	// Remove one
	if err := removeMCPFromConfig(configPath, "github-mcp"); err != nil {
		t.Fatalf("removeMCPFromConfig: %v", err)
	}
	data3, _ := os.ReadFile(configPath)
	var result3 map[string]any
	if err := json.Unmarshal(data3, &result3); err != nil {
		t.Fatalf("parse config3: %v", err)
	}
	servers3 := result3["mcpServers"].(map[string]any)
	if len(servers3) != 1 {
		t.Errorf("expected 1 server after remove, got %d", len(servers3))
	}
}

func TestInstallAndRemoveSkill(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")

	// Create a source skill dir
	srcDir := filepath.Join(dir, "src-skill")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Install skill
	err := installSkillBySymlink(skillsDir, srcDir, "my-skill")
	if err != nil {
		t.Fatalf("installSkillBySymlink error: %v", err)
	}

	// Verify symlink exists
	target := filepath.Join(skillsDir, "my-skill")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}

	// Remove skill
	err = removeSkillDir(skillsDir, "my-skill")
	if err != nil {
		t.Fatalf("removeSkillDir error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("skill should be removed")
	}
}
