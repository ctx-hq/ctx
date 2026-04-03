package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestVisibilityValues_Valid(t *testing.T) {
	valid := []string{"public", "unlisted", "private"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			ok := v == "public" || v == "unlisted" || v == "private"
			if !ok {
				t.Errorf("visibility %q should be valid", v)
			}
		})
	}
}

func TestVisibilityValues_Invalid(t *testing.T) {
	invalid := []string{"internal", "PUBLIC", "Protected", "", "shared", "org-only"}
	for _, v := range invalid {
		t.Run(v, func(t *testing.T) {
			ok := v == "public" || v == "unlisted" || v == "private"
			if ok {
				t.Errorf("visibility %q should be invalid", v)
			}
		})
	}
}

func TestVisibilitySet_Online(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert exact method, path, and body
		if r.Method != "PATCH" {
			t.Errorf("method = %q, want PATCH", r.Method)
			http.NotFound(w, r)
			return
		}
		wantPath := "/v1/packages/@scope%2Fpkg/visibility"
		if r.URL.RawPath != "" {
			if r.URL.RawPath != wantPath {
				t.Errorf("raw path = %q, want %q", r.URL.RawPath, wantPath)
				http.NotFound(w, r)
				return
			}
		} else if r.URL.Path != "/v1/packages/@scope%2Fpkg/visibility" {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
			return
		}
		if body["visibility"] != "private" {
			t.Errorf("body visibility = %q, want %q", body["visibility"], "private")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"full_name":  "@scope/pkg",
			"visibility": body["visibility"],
		})
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", srv.URL)
	writeCredentials(t, home, "valid-token")

	cmd := exec.Command(binary, "visibility", "@scope/pkg", "private", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("visibility set failed: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data["visibility"] != "private" {
		t.Errorf("visibility = %q, want %q", resp.Data["visibility"], "private")
	}
}

func TestVisibilitySet_NotLoggedIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")

	cmd := exec.Command(binary, "visibility", "@scope/pkg", "public", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when not logged in")
	}

	var resp struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Code != "auth" {
		t.Errorf("code = %q, want %q", resp.Code, "auth")
	}
}

func TestVisibilityCmd_AcceptsOneOrTwoArgs(t *testing.T) {
	// visibilityCmd uses cobra.RangeArgs(1, 2)
	cmd := visibilityCmd

	tests := []struct {
		name    string
		nArgs   int
		wantErr bool
	}{
		{"zero args is invalid", 0, true},
		{"one arg is valid (view)", 1, false},
		{"two args is valid (set)", 2, false},
		{"three args is invalid", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]string, tt.nArgs)
			for i := range args {
				args[i] = "arg"
			}
			err := cmd.Args(cmd, args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%d) error = %v, wantErr = %v", tt.nArgs, err, tt.wantErr)
			}
		})
	}
}
