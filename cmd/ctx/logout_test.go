package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogout_LoggedIn(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", "")
	writeCredentials(t, home, "fake-token")

	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logout failed: %v\n%s", err, out)
	}

	var resp struct {
		OK      bool       `json:"ok"`
		Data    LogoutInfo `json:"data"`
		Summary string     `json:"summary"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Status != "logged_out" {
		t.Errorf("status = %q, want %q", resp.Data.Status, "logged_out")
	}
	if resp.Data.Username != "testuser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "testuser")
	}
	if !strings.Contains(resp.Summary, "was: testuser") {
		t.Errorf("summary = %q, want to contain 'was: testuser'", resp.Summary)
	}
}

func TestLogout_AlreadyLoggedOut(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")

	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logout failed: %v\n%s", err, out)
	}

	var resp struct {
		OK      bool       `json:"ok"`
		Data    LogoutInfo `json:"data"`
		Summary string     `json:"summary"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Status != "already_logged_out" {
		t.Errorf("status = %q, want %q", resp.Data.Status, "already_logged_out")
	}
	if !strings.Contains(resp.Summary, "Already logged out") {
		t.Errorf("summary = %q, want to contain 'Already logged out'", resp.Summary)
	}
}

func TestLogout_TokenButNoUsername(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	// Profile with no username but a token
	setupProfile(t, home, "", "")
	writeCredentials(t, home, "orphan-token")

	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logout failed: %v\n%s", err, out)
	}

	var resp struct {
		OK   bool       `json:"ok"`
		Data LogoutInfo `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}

	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Data.Status != "logged_out" {
		t.Errorf("status = %q, want %q", resp.Data.Status, "logged_out")
	}
	if resp.Data.Username != "" {
		t.Errorf("username = %q, want empty", resp.Data.Username)
	}
}

func TestLogout_ClearsCredentials(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", "")
	writeCredentials(t, home, "fake-token")

	// Logout
	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("logout failed: %v\n%s", err, out)
	}

	// Verify credentials file doesn't contain profile:default token
	credPath := filepath.Join(home, "credentials")
	if data, err := os.ReadFile(credPath); err == nil {
		if strings.Contains(string(data), "profile:default") {
			t.Error("credentials file still contains profile token after logout")
		}
	}

	// Verify whoami now fails
	cmd2 := exec.Command(binary, "whoami", "--offline", "--json")
	cmd2.Env = hermeticEnv(home)
	out2, err := cmd2.CombinedOutput()
	if err == nil {
		t.Fatal("expected whoami to fail after logout")
	}

	var resp struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(out2, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out2)
	}

	if resp.OK {
		t.Error("expected ok=false")
	}
	if resp.Code != "auth" {
		t.Errorf("code = %q, want %q", resp.Code, "auth")
	}
}

func TestLogout_Idempotent(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", "")
	writeCredentials(t, home, "fake-token")

	// First logout
	cmd1 := exec.Command(binary, "logout", "--json")
	cmd1.Env = hermeticEnv(home)
	out1, err := cmd1.CombinedOutput()
	if err != nil {
		t.Fatalf("first logout failed: %v\n%s", err, out1)
	}

	var resp1 struct {
		OK   bool       `json:"ok"`
		Data LogoutInfo `json:"data"`
	}
	if err := json.Unmarshal(out1, &resp1); err != nil {
		t.Fatalf("failed to parse first JSON: %v\nraw: %s", err, out1)
	}
	if resp1.Data.Status != "logged_out" {
		t.Errorf("first status = %q, want %q", resp1.Data.Status, "logged_out")
	}

	// Second logout — should be idempotent
	cmd2 := exec.Command(binary, "logout", "--json")
	cmd2.Env = hermeticEnv(home)
	out2, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("second logout failed: %v\n%s", err, out2)
	}

	var resp2 struct {
		OK   bool       `json:"ok"`
		Data LogoutInfo `json:"data"`
	}
	if err := json.Unmarshal(out2, &resp2); err != nil {
		t.Fatalf("failed to parse second JSON: %v\nraw: %s", err, out2)
	}
	if resp2.Data.Status != "already_logged_out" {
		t.Errorf("second status = %q, want %q", resp2.Data.Status, "already_logged_out")
	}
}

func TestLogout_NoArgs(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")

	cmd := exec.Command(binary, "logout", "extraarg")
	cmd.Env = hermeticEnv(home)
	_, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error with extra arguments")
	}
}

func TestLogout_JSONEnvelope(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "envelopeuser", "")
	writeCredentials(t, home, "valid-token")

	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logout --json failed: %v\n%s", err, out)
	}

	var resp struct {
		OK      bool       `json:"ok"`
		Data    LogoutInfo `json:"data"`
		Summary string     `json:"summary"`
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
	if resp.Data.Username != "envelopeuser" {
		t.Errorf("username = %q, want %q", resp.Data.Username, "envelopeuser")
	}
	if resp.Data.Status != "logged_out" {
		t.Errorf("status = %q, want %q", resp.Data.Status, "logged_out")
	}
	if !strings.Contains(resp.Summary, "envelopeuser") {
		t.Errorf("summary = %q, want to contain username", resp.Summary)
	}
	if len(resp.Breadcrumbs) == 0 {
		t.Error("expected at least one breadcrumb")
	}
}

func TestLogout_KeychainReadError(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", "")
	writeCredentials(t, home, "valid-token")

	// Make the credentials file unreadable to simulate keychain access failure.
	credPath := filepath.Join(home, "credentials")
	if err := os.Chmod(credPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(credPath, 0o600) })

	// With unreadable credentials, getToken() returns "", so logout
	// may still succeed (profile exists but token unreadable).
	// The important thing is it doesn't crash.
	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, _ := cmd.CombinedOutput()

	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
}

func TestLogout_KeychainDeleteError(t *testing.T) {
	binary := buildCtxBinary(t)
	home := setupCtxHome(t, "")
	setupProfile(t, home, "testuser", "")
	writeCredentials(t, home, "valid-token")

	// Make the CTX_HOME directory read-only so the fileKeychain cannot
	// remove or rewrite the credentials file, but can still read it.
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	cmd := exec.Command(binary, "logout", "--json")
	cmd.Env = hermeticEnv(home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected logout to fail when keychain delete fails, got: %s", out)
	}

	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if resp.OK {
		t.Error("expected ok=false when keychain delete fails")
	}

	// Verify the token is still present (logout did not silently succeed).
	_ = os.Chmod(home, 0o700) // restore
	credPath := filepath.Join(home, "credentials")
	data, readErr := os.ReadFile(credPath)
	if readErr != nil {
		t.Fatalf("cannot read credentials: %v", readErr)
	}
	if !strings.Contains(string(data), "profile:default") {
		t.Error("token should still be in credentials after failed logout")
	}
}
