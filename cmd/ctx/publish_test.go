package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
)

// requestKey returns a consistent key for identifying API requests,
// using RawPath when available (preserves %2F encoding).
func requestKey(r *http.Request) string {
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}
	return r.Method + " " + path
}

// upgradeTestServer creates a test server that tracks API calls and responds
// based on the given initial package state. Returns the server and a function
// to retrieve the recorded calls.
func upgradeTestServer(t *testing.T, visibility string, exists bool) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var calls []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := requestKey(r)
		mu.Lock()
		calls = append(calls, key)
		mu.Unlock()

		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/packages/"):
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"full_name":  "@test/pkg",
				"type":       "skill",
				"visibility": visibility,
			})
		case r.Method == "PATCH" && strings.HasSuffix(r.URL.Path, "/visibility"):
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"full_name":  "@test/pkg",
				"visibility": body["visibility"],
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	getCalls := func() []string {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]string, len(calls))
		copy(cp, calls)
		return cp
	}
	return srv, getCalls
}

func hasPATCH(calls []string, suffix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, "PATCH") && strings.Contains(c, suffix) {
			return true
		}
	}
	return false
}

func TestMaybeUpgradeVisibility_PrivateToPublic(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "private", true)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/pkg", "public", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	// GET + PATCH visibility
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
	if !hasPATCH(calls, "visibility") {
		t.Error("expected PATCH visibility call")
	}
}

func TestMaybeUpgradeVisibility_PrivateToPublic_DefaultEmpty(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "private", true)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/pkg", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	if !hasPATCH(calls, "visibility") {
		t.Error("expected PATCH visibility call when targetVis is empty (defaults to public)")
	}
}

func TestMaybeUpgradeVisibility_PrivateToUnlisted(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "private", true)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/pkg", "unlisted", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	if !hasPATCH(calls, "visibility") {
		t.Error("expected PATCH visibility call")
	}
}

func TestMaybeUpgradeVisibility_TargetPrivate_NoUpgrade(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "private", true)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/pkg", "private", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls when target is private, got %d: %v", len(calls), calls)
	}
}

func TestMaybeUpgradeVisibility_AlreadyPublic_NoUpgrade(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "public", true)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/pkg", "public", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	// Only GET, no PATCH
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (GET only), got %d: %v", len(calls), calls)
	}
}

func TestMaybeUpgradeVisibility_NewPackage_NoUpgrade(t *testing.T) {
	srv, getCalls := upgradeTestServer(t, "", false)
	defer srv.Close()

	w := output.NewWriter()
	reg := registry.New(srv.URL, "test-token")
	err := maybeUpgradeVisibility(context.Background(), reg, w, "@test/new-pkg", "public", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := getCalls()
	// Only GET (404), no PATCH
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (GET 404), got %d: %v", len(calls), calls)
	}
}
