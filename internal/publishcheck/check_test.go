package publishcheck

import (
	"context"
	"fmt"
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

func TestCheckSource_GitHubPrefix_Skipped(t *testing.T) {
	// github: prefix is declarative metadata, not probed via HTTP.
	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: "github:basecamp/fizzy-cli"},
	}

	results := Check(context.Background(), m)
	for _, r := range results {
		if r.Method == "source" {
			t.Errorf("github: source should not be validated, got result: %+v", r)
		}
	}
}

func TestCheckSource_NpmPrefix_Skipped(t *testing.T) {
	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: "npm:some-package"},
	}

	results := Check(context.Background(), m)
	for _, r := range results {
		if r.Method == "source" {
			t.Errorf("npm: source should not be validated, got result: %+v", r)
		}
	}
}

func TestCheckSource_HTTPS_Validated(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use the TLS server's URL (https://) and its client that trusts the test cert.
	origCheck := checkURLFunc
	checkURLFunc = func(ctx context.Context, method, pkg, url string) CheckResult {
		result := CheckResult{Method: method, Pkg: pkg}
		resp, err := srv.Client().Head(url)
		if err != nil {
			result.Error = fmt.Sprintf("unreachable: %v", err)
			return result
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			return result
		}
		result.OK = true
		return result
	}
	defer func() { checkURLFunc = origCheck }()

	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: srv.URL + "/tool.tar.gz"},
	}

	results := Check(context.Background(), m)
	found := false
	for _, r := range results {
		if r.Method == "source" {
			found = true
			if !r.OK {
				t.Errorf("expected OK for reachable https source, got error: %s", r.Error)
			}
		}
	}
	if !found {
		t.Error("expected https:// source to be validated")
	}
}

func TestCheckSource_HTTPS_404(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origCheck := checkURLFunc
	checkURLFunc = func(ctx context.Context, method, pkg, url string) CheckResult {
		result := CheckResult{Method: method, Pkg: pkg}
		resp, err := srv.Client().Head(url)
		if err != nil {
			result.Error = fmt.Sprintf("unreachable: %v", err)
			return result
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			result.OK = false
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			return result
		}
		result.OK = true
		return result
	}
	defer func() { checkURLFunc = origCheck }()

	m := &manifest.Manifest{
		Type:    manifest.TypeCLI,
		Install: &manifest.InstallSpec{Source: srv.URL + "/missing.tar.gz"},
	}

	results := Check(context.Background(), m)
	found := false
	for _, r := range results {
		if r.Method == "source" {
			found = true
			if r.OK {
				t.Error("expected failure for 404 https source")
			}
		}
	}
	if !found {
		t.Error("expected https:// source to be validated")
	}
}
