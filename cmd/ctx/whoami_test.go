package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	testBinary     string
	testBinaryOnce sync.Once
)

func buildCtxBinary(t *testing.T) string {
	t.Helper()
	testBinaryOnce.Do(func() {
		testBinary = filepath.Join(os.TempDir(), "ctx-whoami-test")
		build := exec.Command("go", "build", "-o", testBinary, "./")
		build.Dir = "."
		if out, err := build.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("build failed: %v\n%s", err, out))
		}
	})
	return testBinary
}

func setupCtxHome(t *testing.T, configYAML string) string {
	t.Helper()
	home := t.TempDir()
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(configYAML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// setupProfile creates a profiles.yaml with a "default" profile.
func setupProfile(t *testing.T, home, username, registryURL string) {
	t.Helper()
	// Always include username field (even if empty) to avoid YAML nil entry
	content := "active: default\nprofiles:\n  default:\n    username: \"" + username + "\"\n"
	if registryURL != "" {
		content += "    registry: " + registryURL + "\n"
	}
	if err := os.WriteFile(filepath.Join(home, "profiles.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeCredentials writes a token in the key=value format expected by fileKeychain.
func writeCredentials(t *testing.T, home, token string) {
	t.Helper()
	content := "ctx-cli:profile:default=" + token + "\n"
	if err := os.WriteFile(filepath.Join(home, "credentials"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// hermericEnv returns an env slice for test subprocesses that isolates them
// from the host keychain. On macOS this removes "security" from PATH so that
// darwinKeychain is not initialized, forcing fallback to fileKeychain.
func hermeticEnv(home string, extra ...string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
	}
	// Provide a minimal PATH: go toolchain + /usr/local/bin + /bin (coreutils).
	// Exclude /usr/bin which contains macOS "security" command.
	gobin := os.Getenv("GOPATH")
	if gobin == "" {
		gobin = filepath.Join(os.Getenv("HOME"), "go")
	}
	minimalPath := gobin + "/bin:/usr/local/go/bin:/usr/local/bin:/bin:/usr/sbin"
	env = append(env, "PATH="+minimalPath)
	env = append(env, "CTX_HOME="+home)
	env = append(env, extra...)
	return env
}

func TestWhoami_NotLoggedIn(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")

	cmd := exec.Command(binary, "whoami", "--json", "--offline")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when not logged in")
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
		Hint  string `json:"hint"`
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
	if !strings.Contains(resp.Error, "not logged in") {
		t.Errorf("error = %q, want to contain 'not logged in'", resp.Error)
	}
}

func TestWhoami_Online(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"username":   "testuser",
				"email":      "test@example.com",
				"avatar_url": "https://example.com/avatar.png",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "registry: "+srv.URL+"\n")
	setupProfile(t, home, "testuser", srv.URL)
	writeCredentials(t, home, "fake-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami --json failed: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool       `json:"ok"`
		Data WhoamiInfo `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Username != "testuser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "testuser")
	}
	if resp.Data.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", resp.Data.Email, "test@example.com")
	}
	if resp.Data.Source != "api" {
		t.Errorf("source = %q, want %q", resp.Data.Source, "api")
	}
}

func TestWhoami_TokenRevoked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "registry: "+srv.URL+"\n")
	setupProfile(t, home, "olduser", srv.URL)
	writeCredentials(t, home, "expired-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for revoked token")
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
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
	if !strings.Contains(resp.Error, "expired or revoked") {
		t.Errorf("error = %q, want to contain 'expired or revoked'", resp.Error)
	}
}

func TestWhoami_Offline(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "cacheduser", "")
	writeCredentials(t, home, "some-token")

	cmd := exec.Command(binary, "whoami", "--offline", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami --offline failed: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool       `json:"ok"`
		Data WhoamiInfo `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Username != "cacheduser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "cacheduser")
	}
	if resp.Data.Email != "" {
		t.Errorf("email should be empty in cached mode, got %q", resp.Data.Email)
	}
}

func TestWhoami_Offline_NoCache(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	writeCredentials(t, home, "some-token")

	cmd := exec.Command(binary, "whoami", "--offline", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when offline with no cached username")
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
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

func TestWhoami_NetworkFallback(t *testing.T) {
	// Use a server that's immediately closed to simulate network failure
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "fallbackuser", srv.URL)
	writeCredentials(t, home, "some-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami should fall back to cached: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool       `json:"ok"`
		Data WhoamiInfo `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Username != "fallbackuser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "fallbackuser")
	}
}

func TestWhoami_NetworkFallback_NoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	// Profile exists but no username cached
	setupProfile(t, home, "", srv.URL)
	writeCredentials(t, home, "some-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when network fails and no cache")
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
	if resp.Code != "network" {
		t.Errorf("code = %q, want %q", resp.Code, "network")
	}
}

func TestWhoami_ServerError_Fallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "An unexpected error occurred"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "cacheduser", srv.URL)
	writeCredentials(t, home, "valid-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami should fall back to cached on 500: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool       `json:"ok"`
		Data WhoamiInfo `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Username != "cacheduser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "cacheduser")
	}
}

func TestWhoami_ServerError_NoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "An unexpected error occurred"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	// Profile exists but no username
	setupProfile(t, home, "", srv.URL)
	writeCredentials(t, home, "valid-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when server 500 and no cache")
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
	if resp.Code != "api" {
		t.Errorf("code = %q, want %q", resp.Code, "api")
	}
}

func TestWhoami_NoArgs(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")

	cmd := exec.Command(binary, "whoami", "extraarg", "--json")
	cmd.Env = hermeticEnv(home)
	_, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error with extra arguments")
	}
}

func TestWhoami_JSONEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"username":   "envelopeuser",
				"email":      "env@example.com",
				"avatar_url": "",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "", srv.URL)
	writeCredentials(t, home, "valid-token")

	cmd := exec.Command(binary, "whoami", "--json")
	cmd.Env = hermeticEnv(home, "CTX_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami --json failed: %v\n%s", err, out)
	}

	// Verify full JSON envelope structure
	var resp struct {
		OK          bool       `json:"ok"`
		Data        WhoamiInfo `json:"data"`
		Summary     string     `json:"summary"`
		Breadcrumbs []struct {
			Action      string `json:"action"`
			Cmd         string `json:"cmd"`
			Description string `json:"description"`
		} `json:"breadcrumbs"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if !strings.Contains(resp.Summary, "envelopeuser") {
		t.Errorf("summary = %q, want to contain username", resp.Summary)
	}
	if len(resp.Breadcrumbs) == 0 {
		t.Error("expected at least one breadcrumb")
	}
	if resp.Breadcrumbs[0].Action != "switch" {
		t.Errorf("first breadcrumb action = %q, want %q", resp.Breadcrumbs[0].Action, "switch")
	}
}
