package mcpserver

import (
	"testing"
)

func TestToolDefinitions_AllRegistered(t *testing.T) {
	s := New()

	expectedTools := []string{
		"ctx_search", "ctx_install", "ctx_info", "ctx_list",
		"ctx_remove", "ctx_update", "ctx_outdated", "ctx_doctor", "ctx_agents",
	}

	for _, name := range expectedTools {
		if _, ok := s.tools[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
		if _, ok := toolDefinitions[name]; !ok {
			t.Errorf("tool %q has no definition", name)
		}
	}
}

func TestToolDefinitions_HaveRequiredFields(t *testing.T) {
	for name, def := range toolDefinitions {
		if def["name"] == nil {
			t.Errorf("tool %q missing name", name)
		}
		if def["description"] == nil {
			t.Errorf("tool %q missing description", name)
		}
		if def["inputSchema"] == nil {
			t.Errorf("tool %q missing inputSchema", name)
		}

		schema, ok := def["inputSchema"].(map[string]any)
		if !ok {
			t.Errorf("tool %q inputSchema is not a map", name)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("tool %q inputSchema.type = %q, want \"object\"", name, schema["type"])
		}
	}
}

func TestToolDefinitions_SearchHasPagination(t *testing.T) {
	def := toolDefinitions["ctx_search"]
	schema := def["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)

	if _, ok := props["limit"]; !ok {
		t.Error("ctx_search should have limit parameter")
	}
	if _, ok := props["offset"]; !ok {
		t.Error("ctx_search should have offset parameter")
	}
}

func TestToolDefinitions_RemoveRequiresPackage(t *testing.T) {
	def := toolDefinitions["ctx_remove"]
	schema := def["inputSchema"].(map[string]any)
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("ctx_remove should have required fields")
	}

	found := false
	for _, r := range required {
		if r == "package" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ctx_remove should require 'package' parameter")
	}
}

func TestHandleDoctor_ReturnsResult(t *testing.T) {
	result, err := handleDoctor(nil, nil)
	if err != nil {
		t.Fatalf("handleDoctor error: %v", err)
	}
	if result == nil {
		t.Fatal("handleDoctor returned nil")
	}
}

func TestHandleAgents_ReturnsResult(t *testing.T) {
	result, err := handleAgents(nil, nil)
	if err != nil {
		t.Fatalf("handleAgents error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("handleAgents should return map")
	}
	if _, ok := m["agents"]; !ok {
		t.Error("result should contain 'agents' key")
	}
	if _, ok := m["total"]; !ok {
		t.Error("result should contain 'total' key")
	}
}
