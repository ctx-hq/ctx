package publishcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
)

func TestCheckScriptURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Script: srv.URL + "/install.sh"},
	}

	results := Check(context.Background(), m)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].OK {
		t.Errorf("expected OK for reachable URL, got error: %s", results[0].Error)
	}
	if results[0].Method != "script" {
		t.Errorf("Method = %q, want script", results[0].Method)
	}
}

func TestCheckScriptURLUnreachable(t *testing.T) {
	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Script: "https://localhost:1/nonexistent.sh"},
	}

	results := Check(context.Background(), m)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected failure for unreachable URL")
	}
}

func TestCheckNoInstallSpec(t *testing.T) {
	m := &manifest.Manifest{
		Type: manifest.TypeSkill,
	}

	results := Check(context.Background(), m)
	if len(results) != 0 {
		t.Errorf("expected 0 results for no install spec, got %d", len(results))
	}
}

func TestCheckScriptURL404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Script: srv.URL + "/missing.sh"},
	}

	results := Check(context.Background(), m)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected failure for 404 URL")
	}
	if results[0].Error != "HTTP 404" {
		t.Errorf("Error = %q, want 'HTTP 404'", results[0].Error)
	}
}

func TestCheckSourceGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Can't test real GitHub API, but test that github: prefix is recognized
	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: "github:basecamp/fizzy-cli"},
	}

	results := Check(context.Background(), m)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Method != "source" {
		t.Errorf("Method = %q, want source", results[0].Method)
	}
	// Will fail because it's hitting real GitHub API, but should not be "unrecognized"
	if results[0].Error == "unrecognized source prefix (expected github:, npm:, pip:, or https://)" {
		t.Error("github: prefix should be recognized")
	}
}

func TestCheckSourceUnrecognizedPrefix(t *testing.T) {
	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: "ftp://bad-source"},
	}

	results := Check(context.Background(), m)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected failure for unrecognized prefix")
	}
	if results[0].Error != "unrecognized source prefix (expected github:, npm:, pip:, or https://)" {
		t.Errorf("Error = %q, want unrecognized prefix error", results[0].Error)
	}
}
