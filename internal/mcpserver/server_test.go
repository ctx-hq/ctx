package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ctx-hq/ctx/internal/mcpproto"
)

func TestHandleInitialize(t *testing.T) {
	s := New()
	req := &mcpproto.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := s.handleRequest(context.Background(), req)
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["protocolVersion"] != "2025-11-05" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if serverInfo["name"] != "ctx" {
		t.Errorf("serverInfo.name = %v", serverInfo["name"])
	}
}

func TestHandleToolsList(t *testing.T) {
	s := New()
	req := &mcpproto.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := s.handleRequest(context.Background(), req)
	if resp == nil {
		t.Fatal("response is nil")
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatal("tools is not an array")
	}

	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool["name"].(string)] = true
	}
	for _, expected := range []string{"ctx_search", "ctx_install", "ctx_info", "ctx_list", "ctx_remove", "ctx_update", "ctx_outdated", "ctx_doctor", "ctx_agents"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	s := New()
	req := &mcpproto.Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "unknown/method",
	}

	resp := s.handleRequest(context.Background(), req)
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
}

func TestHandleToolsCallUnknown(t *testing.T) {
	s := New()
	params, _ := json.Marshal(map[string]any{
		"name":      "nonexistent_tool",
		"arguments": map[string]any{},
	})
	req := &mcpproto.Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(context.Background(), req)
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Error == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s := New()
	req := &mcpproto.Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	resp := s.handleRequest(context.Background(), req)
	if resp != nil {
		t.Error("notifications should not produce a response")
	}
}
