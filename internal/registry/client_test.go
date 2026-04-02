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

func TestClientPublish_PathAndMethod(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(PublishResponse{
			FullName: "@test/pkg",
			Version:  "1.0.0",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	resp, err := c.Publish(context.Background(), []byte("name: test"), nil, nil)
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/packages" {
		t.Errorf("path = %q, want /v1/packages", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", gotContentType)
	}
	if resp.FullName != "@test/pkg" || resp.Version != "1.0.0" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestClientYank_PathAndMethod(t *testing.T) {
	var gotMethod string
	var gotRawPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRawPath = r.URL.RawPath
		if gotRawPath == "" {
			gotRawPath = r.URL.Path
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	err := c.Yank(context.Background(), "@test/pkg", "1.0.0")
	if err != nil {
		t.Fatalf("Yank error: %v", err)
	}
	if gotMethod != "PATCH" {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	wantPath := "/v1/packages/@test%2Fpkg/versions/1.0.0"
	if gotRawPath != wantPath {
		t.Errorf("raw path = %q, want %q", gotRawPath, wantPath)
	}
	if yanked, ok := gotBody["yanked"]; !ok || yanked != true {
		t.Errorf("body = %v, want {yanked: true}", gotBody)
	}
}

func TestClientDeletePackage_PathAndMethod(t *testing.T) {
	var gotMethod, gotRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRawPath = r.URL.RawPath
		if gotRawPath == "" {
			gotRawPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"full_name": "@test/pkg", "deleted": true})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	err := c.DeletePackage(context.Background(), "@test/pkg")
	if err != nil {
		t.Fatalf("DeletePackage error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	wantPath := "/v1/packages/@test%2Fpkg"
	if gotRawPath != wantPath {
		t.Errorf("raw path = %q, want %q", gotRawPath, wantPath)
	}
}

func TestClientDeleteVersion_PathAndMethod(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		version  string
		wantPath string
	}{
		{"simple version", "@test/pkg", "1.0.0", "/v1/packages/@test%2Fpkg/versions/1.0.0"},
		{"version with plus", "@test/pkg", "2.0.0+build.123", "/v1/packages/@test%2Fpkg/versions/2.0.0+build.123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotRawPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotRawPath = r.URL.RawPath
				if gotRawPath == "" {
					gotRawPath = r.URL.Path
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"full_name": tt.pkg, "version": tt.version, "deleted": true})
			}))
			defer srv.Close()

			c := New(srv.URL, "tok")
			err := c.DeleteVersion(context.Background(), tt.pkg, tt.version)
			if err != nil {
				t.Fatalf("DeleteVersion error: %v", err)
			}
			if gotMethod != "DELETE" {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if gotRawPath != tt.wantPath {
				t.Errorf("raw path = %q, want %q", gotRawPath, tt.wantPath)
			}
		})
	}
}

func TestClientDownload_PathAndMethod(t *testing.T) {
	var gotMethod, gotRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRawPath = r.URL.RawPath
		if gotRawPath == "" {
			gotRawPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("archive-data"))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	rc, err := c.Download(context.Background(), "@test/pkg", "1.0.0")
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	defer rc.Close()

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	wantPath := "/v1/packages/@test%2Fpkg/versions/1.0.0/archive"
	if gotRawPath != wantPath {
		t.Errorf("raw path = %q, want %q", gotRawPath, wantPath)
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
