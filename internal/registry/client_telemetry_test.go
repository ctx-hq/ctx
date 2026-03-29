package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportInstall_SendsCorrectPayload(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/telemetry/install" {
			t.Errorf("path = %q, want /v1/telemetry/install", r.URL.Path)
		}

		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	c.ReportInstall(context.Background(), "@test/skill", "1.0.0", []string{"claude", "cursor"}, "registry")

	if gotBody["package"] != "@test/skill" {
		t.Errorf("package = %v, want %q", gotBody["package"], "@test/skill")
	}
	if gotBody["version"] != "1.0.0" {
		t.Errorf("version = %v, want %q", gotBody["version"], "1.0.0")
	}
	if gotBody["source_type"] != "registry" {
		t.Errorf("source_type = %v, want %q", gotBody["source_type"], "registry")
	}

	agents, ok := gotBody["agents"].([]interface{})
	if !ok {
		t.Fatalf("agents is not an array: %T", gotBody["agents"])
	}
	if len(agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(agents))
	}
}

func TestReportInstall_DoesNotPanic_OnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	// ReportInstall is fire-and-forget; should not panic on error
	c.ReportInstall(context.Background(), "@test/skill", "1.0.0", nil, "registry")
}

func TestReportInstall_EmptyAgents(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	c.ReportInstall(context.Background(), "@test/skill", "1.0.0", []string{}, "github")

	if gotBody["source_type"] != "github" {
		t.Errorf("source_type = %v, want %q", gotBody["source_type"], "github")
	}
}
