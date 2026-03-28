package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestClientSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q != "test" {
			t.Errorf("q = %q, want %q", q, "test")
		}

		_ = json.NewEncoder(w).Encode(SearchResult{
			Packages: []PackageInfo{
				{FullName: "@test/pkg", Type: "skill", Description: "A test"},
			},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	result, err := c.Search(context.Background(), "test", "", "", 20)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if result.Packages[0].FullName != "@test/pkg" {
		t.Errorf("FullName = %q", result.Packages[0].FullName)
	}
}

func TestClientGetPackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(PackageDetail{
			PackageInfo: PackageInfo{
				FullName:    "@test/pkg",
				Type:        "mcp",
				Description: "test mcp",
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	pkg, err := c.GetPackage(context.Background(), "@test/pkg")
	if err != nil {
		t.Fatalf("GetPackage error: %v", err)
	}
	if pkg.FullName != "@test/pkg" {
		t.Errorf("FullName = %q", pkg.FullName)
	}
}

func TestClientAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token-123")
	_, _ = c.GetMe(context.Background())

	if gotAuth != "Bearer test-token-123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token-123")
	}
}

func TestClientUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_ = json.NewEncoder(w).Encode(SearchResult{})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, _ = c.Search(context.Background(), "test", "", "", 10)

	if !strings.HasPrefix(gotUA, "ctx/") {
		t.Errorf("User-Agent = %q, should start with ctx/", gotUA)
	}
	if !strings.Contains(gotUA, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("User-Agent = %q, should contain %s/%s", gotUA, runtime.GOOS, runtime.GOARCH)
	}
}

func TestClientErrorParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "not_found",
			Message: "Package not found",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.GetPackage(context.Background(), "@test/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "API error (404): Package not found" {
		t.Errorf("error = %q", err.Error())
	}
}
