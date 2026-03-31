package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenELFDetected_WithDir(t *testing.T) {
	dir := t.TempDir()
	openelfDir := filepath.Join(dir, ".openelf")
	if err := os.MkdirAll(openelfDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &openelfAgent{home: dir}
	if !a.Detected() {
		t.Error("expected Detected() = true when ~/.openelf exists")
	}
}

func TestOpenELFDetected_WithEnvVar(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "custom-openelf")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENELF_HOME", customDir)

	// Use a home dir that does NOT have .openelf — env should take precedence
	a := &openelfAgent{home: filepath.Join(dir, "nonexistent")}
	if !a.Detected() {
		t.Error("expected Detected() = true with OPENELF_HOME set")
	}
}

func TestOpenELFDetected_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	// Ensure no OPENELF_HOME env is set
	t.Setenv("OPENELF_HOME", "")

	a := &openelfAgent{home: dir}
	if a.Detected() {
		t.Error("expected Detected() = false when ~/.openelf does not exist")
	}
}

func TestOpenELFSkillsDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENELF_HOME", "")

	a := &openelfAgent{home: dir}
	want := filepath.Join(dir, ".openelf", "skills")
	if got := a.SkillsDir(); got != want {
		t.Errorf("SkillsDir() = %q, want %q", got, want)
	}
}

func TestOpenELFSkillsDir_WithEnvVar(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "my-openelf")
	t.Setenv("OPENELF_HOME", customDir)

	a := &openelfAgent{home: dir}
	want := filepath.Join(customDir, "skills")
	if got := a.SkillsDir(); got != want {
		t.Errorf("SkillsDir() = %q, want %q", got, want)
	}
}

func TestOpenELFAddMCP_NewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	cfg := MCPConfig{
		Command: "npx",
		Args:    []string{"-y", "@mcp/github"},
		Env:     map[string]string{"GITHUB_TOKEN": "test"},
	}
	if err := writeOpenELFMCP(configPath, "github", cfg); err != nil {
		t.Fatalf("writeOpenELFMCP: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Check version
	if v, _ := raw["version"].(float64); v != 2 {
		t.Errorf("version = %v, want 2", raw["version"])
	}

	// Check servers array
	servers, ok := raw["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("expected 1 server, got %v", raw["servers"])
	}

	entry := servers[0].(map[string]interface{})
	if entry["name"] != "github" {
		t.Errorf("name = %v, want github", entry["name"])
	}
	if entry["command"] != "npx" {
		t.Errorf("command = %v, want npx", entry["command"])
	}
	if entry["enabled"] != true {
		t.Errorf("enabled = %v, want true", entry["enabled"])
	}
	if entry["source"] != "ctx" {
		t.Errorf("source = %v, want ctx", entry["source"])
	}

	// Check args
	args, ok := entry["args"].([]interface{})
	if !ok || len(args) != 2 {
		t.Errorf("args = %v, want 2 elements", entry["args"])
	}

	// Check env
	env, ok := entry["env"].(map[string]interface{})
	if !ok || env["GITHUB_TOKEN"] != "test" {
		t.Errorf("env = %v, want GITHUB_TOKEN=test", entry["env"])
	}
}

func TestOpenELFAddMCP_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	// Write initial file with one server
	initial := `{
  "version": 2,
  "default_timeout": "30s",
  "servers": [
    {"name": "existing", "command": "node", "args": ["server.js"], "enabled": true, "source": "manual"}
  ]
}`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	// Add a second server
	cfg := MCPConfig{Command: "npx", Args: []string{"-y", "@mcp/brave"}}
	if err := writeOpenELFMCP(configPath, "brave", cfg); err != nil {
		t.Fatalf("writeOpenELFMCP: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// Existing top-level field preserved
	if raw["default_timeout"] != "30s" {
		t.Errorf("default_timeout lost: %v", raw["default_timeout"])
	}

	servers := raw["servers"].([]interface{})
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	// First server preserved
	s0 := servers[0].(map[string]interface{})
	if s0["name"] != "existing" || s0["source"] != "manual" {
		t.Errorf("existing server mutated: %v", s0)
	}

	// Second server added
	s1 := servers[1].(map[string]interface{})
	if s1["name"] != "brave" || s1["source"] != "ctx" {
		t.Errorf("new server wrong: %v", s1)
	}
}

func TestOpenELFAddMCP_Upsert(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	// Write initial with a ctx-managed server
	initial := `{
  "version": 2,
  "servers": [
    {"name": "github", "command": "old-cmd", "args": ["old"], "enabled": false, "source": "ctx", "timeout": "60s"}
  ]
}`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	// Upsert with new command
	cfg := MCPConfig{Command: "npx", Args: []string{"-y", "@mcp/github"}}
	if err := writeOpenELFMCP(configPath, "github", cfg); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	servers := raw["servers"].([]interface{})
	if len(servers) != 1 {
		t.Fatalf("expected 1 server (upsert, not duplicate), got %d", len(servers))
	}

	entry := servers[0].(map[string]interface{})
	if entry["command"] != "npx" {
		t.Errorf("command not updated: %v", entry["command"])
	}
	if entry["enabled"] != true {
		t.Errorf("enabled not updated: %v", entry["enabled"])
	}
	// timeout should be preserved (we don't manage it)
	if entry["timeout"] != "60s" {
		t.Errorf("timeout lost: %v", entry["timeout"])
	}
}

func TestOpenELFAddMCP_PreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	// File with unknown top-level and per-server fields
	initial := `{
  "version": 2,
  "custom_field": "keep_me",
  "servers": [
    {"name": "s1", "command": "cmd", "custom_server_field": 42}
  ]
}`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := MCPConfig{Command: "new-cmd"}
	if err := writeOpenELFMCP(configPath, "s2", cfg); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	// Top-level unknown field
	if raw["custom_field"] != "keep_me" {
		t.Errorf("top-level custom field lost: %v", raw["custom_field"])
	}

	// Per-server unknown field
	servers := raw["servers"].([]interface{})
	s0 := servers[0].(map[string]interface{})
	if s0["custom_server_field"] != float64(42) {
		t.Errorf("per-server custom field lost: %v", s0["custom_server_field"])
	}
}

func TestOpenELFAddMCP_URLOnly(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	cfg := MCPConfig{URL: "http://localhost:3000/mcp"}
	if err := writeOpenELFMCP(configPath, "local-http", cfg); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	servers := raw["servers"].([]interface{})
	entry := servers[0].(map[string]interface{})
	if entry["url"] != "http://localhost:3000/mcp" {
		t.Errorf("url = %v", entry["url"])
	}
	if _, hasCmd := entry["command"]; hasCmd {
		t.Error("should not have command when URL-only")
	}
}

func TestOpenELFRemoveMCP(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	initial := `{
  "version": 2,
  "servers": [
    {"name": "a", "command": "a-cmd"},
    {"name": "b", "command": "b-cmd"},
    {"name": "c", "command": "c-cmd"}
  ]
}`
	os.WriteFile(configPath, []byte(initial), 0o600)

	if err := removeOpenELFMCP(configPath, "b"); err != nil {
		t.Fatalf("removeOpenELFMCP: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	servers := raw["servers"].([]interface{})
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers after remove, got %d", len(servers))
	}

	names := make([]string, len(servers))
	for i, s := range servers {
		names[i] = s.(map[string]interface{})["name"].(string)
	}
	if names[0] != "a" || names[1] != "c" {
		t.Errorf("remaining servers = %v, want [a, c]", names)
	}
}

func TestOpenELFRemoveMCP_NonExistent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")

	initial := `{"version": 2, "servers": [{"name": "a", "command": "cmd"}]}`
	os.WriteFile(configPath, []byte(initial), 0o600)

	// Removing a non-existent server should not error
	if err := removeOpenELFMCP(configPath, "does-not-exist"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Original server should still be there
	data, _ := os.ReadFile(configPath)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	servers := raw["servers"].([]interface{})
	if len(servers) != 1 {
		t.Errorf("expected 1 server unchanged, got %d", len(servers))
	}
}

func TestOpenELFRemoveMCP_FileNotExist(t *testing.T) {
	if err := removeOpenELFMCP("/nonexistent/path/mcp.json", "anything"); err != nil {
		t.Errorf("expected nil when file doesn't exist, got: %v", err)
	}
}

func TestOpenELFInstallAndRemoveSkill(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENELF_HOME", "")

	a := &openelfAgent{home: dir}

	// Create .openelf dir and source skill
	openelfDir := filepath.Join(dir, ".openelf")
	if err := os.MkdirAll(openelfDir, 0o755); err != nil {
		t.Fatal(err)
	}

	srcDir := filepath.Join(dir, "src-skill")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Test Skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Install
	if err := a.InstallSkill(srcDir, "my-skill"); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	target := filepath.Join(a.SkillsDir(), "my-skill")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("skill not found after install: %v", err)
	}

	// Verify content accessible
	data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		t.Fatalf("read through link: %v", err)
	}
	if string(data) != "# Test Skill" {
		t.Errorf("content = %q, want '# Test Skill'", string(data))
	}

	// Remove
	if err := a.RemoveSkill("my-skill"); err != nil {
		t.Fatalf("RemoveSkill: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("skill should be removed")
	}
}

func TestOpenELFAddMCP_ViaAgentInterface(t *testing.T) {
	dir := t.TempDir()
	openelfDir := filepath.Join(dir, ".openelf")
	if err := os.MkdirAll(openelfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENELF_HOME", "")

	a := &openelfAgent{home: dir}

	cfg := MCPConfig{Command: "npx", Args: []string{"-y", "@mcp/test"}}
	if err := a.AddMCP("test-server", cfg); err != nil {
		t.Fatalf("AddMCP: %v", err)
	}

	// Verify file is in the right place
	mcpPath := filepath.Join(openelfDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("mcp.json not created: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	servers := raw["servers"].([]interface{})
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	entry := servers[0].(map[string]interface{})
	if entry["name"] != "test-server" {
		t.Errorf("name = %v", entry["name"])
	}

	// Remove via interface
	if err := a.RemoveMCP("test-server"); err != nil {
		t.Fatalf("RemoveMCP: %v", err)
	}

	data, _ = os.ReadFile(mcpPath)
	var raw2 map[string]interface{}
	json.Unmarshal(data, &raw2)
	if rawServers, ok := raw2["servers"]; ok {
		if arr, ok := rawServers.([]interface{}); ok && len(arr) != 0 {
			t.Errorf("expected servers key removed or empty after remove, got %d", len(arr))
		}
	}
}

func TestOpenELFFindByName(t *testing.T) {
	a, err := FindByName("openelf")
	if err != nil {
		t.Fatalf("FindByName(openelf): %v", err)
	}
	if a.Name() != "openelf" {
		t.Errorf("Name() = %q, want openelf", a.Name())
	}
}
