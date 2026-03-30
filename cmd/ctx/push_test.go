package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/pushstate"
	"github.com/ctx-hq/ctx/internal/registry"
)

// --- Push defaults tests (preserved from original) ---

func TestPushDefaults_PrivateAndMutable(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "@test/my-skill",
		Version: "0.1.0",
		Type:    manifest.TypeSkill,
	}
	if m.Visibility == "" {
		m.Visibility = "private"
	}
	if m.Visibility == "private" {
		m.Mutable = true
	}
	if m.Visibility != "private" {
		t.Errorf("Visibility = %q, want %q", m.Visibility, "private")
	}
	if !m.Mutable {
		t.Error("Mutable should be true for private push")
	}
}

func TestPushDefaults_PreservesExplicitVisibility(t *testing.T) {
	m := &manifest.Manifest{
		Name:       "@test/my-skill",
		Version:    "0.1.0",
		Type:       manifest.TypeSkill,
		Visibility: "public",
	}
	if m.Visibility == "" {
		m.Visibility = "private"
	}
	if m.Visibility == "private" {
		m.Mutable = true
	}
	if m.Visibility != "public" {
		t.Errorf("Visibility = %q, want %q", m.Visibility, "public")
	}
	if m.Mutable {
		t.Error("Mutable should remain false for non-private push")
	}
}

func TestPushScopeAutoFill(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		username string
		want     string
	}{
		{"fills placeholder scope", "@your-scope/my-skill", "alice", "@alice/my-skill"},
		{"preserves existing scope", "@bob/my-skill", "alice", "@bob/my-skill"},
		{"fills empty scope", "my-skill", "alice", "@alice/my-skill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &manifest.Manifest{Name: tt.initial}
			scope := m.Scope()
			if scope == "your-scope" || scope == "" {
				if tt.username != "" {
					_, name := manifest.ParseFullName(m.Name)
					m.Name = manifest.FormatFullName(tt.username, name)
				}
			}
			if m.Name != tt.want {
				t.Errorf("Name = %q, want %q", m.Name, tt.want)
			}
		})
	}
}

// --- resolveInput tests ---

func TestResolveInput_NoArgs_NoCtxYaml(t *testing.T) {
	// In a temp dir without ctx.yaml → scan mode.
	orig, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	mode, _ := resolveInput(nil)
	if mode != pushModeScan {
		t.Errorf("mode = %d, want pushModeScan (%d)", mode, pushModeScan)
	}
}

func TestResolveInput_NoArgs_WithCtxYaml(t *testing.T) {
	orig, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "ctx.yaml"), []byte("name: test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	mode, dir := resolveInput(nil)
	if mode != pushModeDir {
		t.Errorf("mode = %d, want pushModeDir (%d)", mode, pushModeDir)
	}
	if dir != "." {
		t.Errorf("dir = %q, want %q", dir, ".")
	}
}

func TestResolveInput_DotArg(t *testing.T) {
	mode, dir := resolveInput([]string{"."})
	if mode != pushModeDir {
		t.Errorf("mode = %d, want pushModeDir", mode)
	}
	if dir != "." {
		t.Errorf("dir = %q, want %q", dir, ".")
	}
}

func TestResolveInput_PathArg(t *testing.T) {
	mode, dir := resolveInput([]string{"./my-skill"})
	if mode != pushModeDir {
		t.Errorf("mode = %d, want pushModeDir", mode)
	}
	if dir != "./my-skill" {
		t.Errorf("dir = %q, want %q", dir, "./my-skill")
	}
}

func TestResolveInput_AbsPathArg(t *testing.T) {
	mode, dir := resolveInput([]string{"/tmp/my-skill"})
	if mode != pushModeDir {
		t.Errorf("mode = %d, want pushModeDir", mode)
	}
	if dir != "/tmp/my-skill" {
		t.Errorf("dir = %q, want %q", dir, "/tmp/my-skill")
	}
}

func TestResolveInput_MDFile(t *testing.T) {
	tmp := t.TempDir()
	mdFile := filepath.Join(tmp, "gc.md")
	if err := os.WriteFile(mdFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	mode, _ := resolveInput([]string{mdFile})
	if mode != pushModeFile {
		t.Errorf("mode = %d, want pushModeFile", mode)
	}
}

func TestResolveInput_ExistingDir(t *testing.T) {
	tmp := t.TempDir()
	mode, dir := resolveInput([]string{tmp})
	if mode != pushModeDir {
		t.Errorf("mode = %d, want pushModeDir", mode)
	}
	if dir != tmp {
		t.Errorf("dir = %q, want %q", dir, tmp)
	}
}

func TestResolveInput_BareName(t *testing.T) {
	mode, _ := resolveInput([]string{"gc"})
	if mode != pushModeName {
		t.Errorf("mode = %d, want pushModeName (%d)", mode, pushModeName)
	}
}

// --- resolveSkillByName tests ---

func TestResolveSkillByName_Found(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create a skill.
	skillDir := filepath.Join(tmp, "skills", "alice", "gc")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestManifest(t, skillDir, "@alice/gc", "0.1.0")

	cfg := &config.Config{Username: "alice"}
	dir, err := resolveSkillByName("gc", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dir != skillDir {
		t.Errorf("dir = %q, want %q", dir, skillDir)
	}
}

func TestResolveSkillByName_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Username: "alice"}
	_, err := resolveSkillByName("nonexistent", cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	cliErr := output.AsCLIError(err)
	if cliErr == nil || cliErr.Code != output.CodeNotFound {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestResolveSkillByName_Ambiguous(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create same skill name in two scopes.
	for _, scope := range []string{"alice", "bob"} {
		dir := filepath.Join(tmp, "skills", scope, "gc")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestManifest(t, dir, "@"+scope+"/gc", "0.1.0")
	}

	cfg := &config.Config{Username: ""}
	_, err := resolveSkillByName("gc", cfg)
	if err == nil {
		t.Fatal("expected error for ambiguous skill")
	}
	cliErr := output.AsCLIError(err)
	if cliErr == nil || cliErr.Code != output.CodeAmbiguous {
		t.Errorf("expected ambiguous error, got %v", err)
	}
}

func TestResolveSkillByName_FullName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	skillDir := filepath.Join(tmp, "skills", "alice", "gc")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestManifest(t, skillDir, "@alice/gc", "0.1.0")

	cfg := &config.Config{Username: ""}
	dir, err := resolveSkillByName("alice/gc", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dir != skillDir {
		t.Errorf("dir = %q, want %q", dir, skillDir)
	}
}

// --- scanSkills tests ---

func TestScanSkills_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}
	skills, err := scanSkills(ps)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestScanSkills_NoDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)
	// No skills directory at all.
	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}
	skills, err := scanSkills(ps)
	if err != nil {
		t.Fatal(err)
	}
	if skills != nil {
		t.Errorf("expected nil skills, got %v", skills)
	}
}

func TestScanSkills_WithDirtyAndClean(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create two skills.
	for _, name := range []string{"clean", "dirty"} {
		dir := filepath.Join(tmp, "skills", "alice", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestManifest(t, dir, "@alice/"+name, "0.1.0")
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Record "clean" as already pushed with current hash.
	cleanDir := filepath.Join(tmp, "skills", "alice", "clean")
	cleanHash, _ := pushstate.HashDir(cleanDir)
	ps := &pushstate.PushState{
		Version: 1,
		Skills: map[string]*pushstate.SkillState{
			"@alice/clean": {LastPushedHash: cleanHash, LastVersion: "0.1.0"},
		},
	}

	skills, err := scanSkills(ps)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	dirtyCount := 0
	for _, s := range skills {
		if s.Dirty {
			dirtyCount++
		}
	}
	if dirtyCount != 1 {
		t.Errorf("expected 1 dirty skill, got %d", dirtyCount)
	}
}

func TestScanSkills_SkipsInvalidManifests(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create a skill with invalid ctx.yaml.
	dir := filepath.Join(tmp, "skills", "alice", "broken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), []byte("not: valid: yaml: {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}
	skills, err := scanSkills(ps)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill entry, got %d", len(skills))
	}
	if skills[0].Error == "" {
		t.Error("expected error for invalid manifest")
	}
}

// --- stageAndArchive tests ---

func TestStageAndArchive_CreatesValidArchive(t *testing.T) {
	dir := t.TempDir()
	writeTestManifest(t, dir, "@test/archive", "0.1.0")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "ctx.yaml"))
	archive, cleanup, err := stageAndArchive(dir, loadTestManifest(t, dir), data)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Archive should be readable.
	buf := make([]byte, 100)
	n, readErr := archive.Read(buf)
	if readErr != nil && readErr != io.EOF {
		t.Fatalf("archive read error: %v", readErr)
	}
	if n == 0 {
		t.Error("archive should not be empty")
	}

	// Should support seeking.
	_, seekErr := archive.Seek(0, io.SeekStart)
	if seekErr != nil {
		t.Fatalf("archive seek error: %v", seekErr)
	}
}

// testRetryConfig uses minimal backoff to avoid slow tests.
var testRetryConfig = retryConfig{
	MaxRetries:     3,
	InitialBackoff: 1 * time.Millisecond,
	MaxBackoff:     5 * time.Millisecond,
	BackoffFactor:  2.0,
}

// --- publishWithRetry tests ---

func TestPublishWithRetry_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registry.PublishResponse{
			FullName: "@test/success",
			Version:  "0.1.0",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestManifest(t, dir, "@test/success", "0.1.0")
	data, _ := os.ReadFile(filepath.Join(dir, "ctx.yaml"))

	archive, cleanup, _ := stageAndArchive(dir, loadTestManifest(t, dir), data)
	defer cleanup()

	reg := registry.New(srv.URL, "test-token")
	result, err := publishWithRetry(context.Background(), reg, data, archive, testRetryConfig)
	if err != nil {
		t.Fatal(err)
	}
	if result.FullName != "@test/success" {
		t.Errorf("FullName = %q", result.FullName)
	}
}

func TestPublishWithRetry_RetriesOnServerError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "server_error",
				"message": "temporary failure",
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registry.PublishResponse{
			FullName: "@test/retry",
			Version:  "0.1.0",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestManifest(t, dir, "@test/retry", "0.1.0")
	data, _ := os.ReadFile(filepath.Join(dir, "ctx.yaml"))

	archive, cleanup, _ := stageAndArchive(dir, loadTestManifest(t, dir), data)
	defer cleanup()

	reg := registry.New(srv.URL, "test-token")
	result, err := publishWithRetry(context.Background(), reg, data, archive, testRetryConfig)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result.FullName != "@test/retry" {
		t.Errorf("FullName = %q", result.FullName)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestPublishWithRetry_NonRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "forbidden",
			"message": "not authorized",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestManifest(t, dir, "@test/forbidden", "0.1.0")
	data, _ := os.ReadFile(filepath.Join(dir, "ctx.yaml"))

	archive, cleanup, _ := stageAndArchive(dir, loadTestManifest(t, dir), data)
	defer cleanup()

	reg := registry.New(srv.URL, "test-token")
	_, err := publishWithRetry(context.Background(), reg, data, archive, testRetryConfig)
	if err == nil {
		t.Fatal("expected error for forbidden")
	}
}

func TestPublishWithRetry_ExhaustsRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "server_error",
			"message": "always fails",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeTestManifest(t, dir, "@test/exhaust", "0.1.0")
	data, _ := os.ReadFile(filepath.Join(dir, "ctx.yaml"))

	archive, cleanup, _ := stageAndArchive(dir, loadTestManifest(t, dir), data)
	defer cleanup()

	reg := registry.New(srv.URL, "test-token")
	_, err := publishWithRetry(context.Background(), reg, data, archive, testRetryConfig)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Errorf("error should mention retry exhaustion, got: %v", err)
	}
	if got := attempts.Load(); got != int32(testRetryConfig.MaxRetries+1) {
		t.Errorf("expected %d attempts, got %d", testRetryConfig.MaxRetries+1, got)
	}
}

// --- calcBackoff tests ---

func TestCalcBackoff(t *testing.T) {
	rc := defaultRetryConfig
	b1 := calcBackoff(1, rc)
	b2 := calcBackoff(2, rc)
	b3 := calcBackoff(3, rc)

	// First backoff should be around 500ms (+ up to 25% jitter).
	if b1 < rc.InitialBackoff || b1 > rc.InitialBackoff*5/4+time.Millisecond {
		t.Errorf("backoff(1) = %v, expected ~500ms", b1)
	}
	// Backoffs should grow.
	if b2 <= b1/2 {
		t.Errorf("backoff(2)=%v should be larger than half of backoff(1)=%v", b2, b1)
	}
	// Should not exceed maxBackoff + jitter.
	if b3 > rc.MaxBackoff*5/4+time.Millisecond {
		t.Errorf("backoff(3) = %v, exceeds max", b3)
	}
}

// --- isRetryable tests ---

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"network error", output.ErrNetwork(nil), true},
		{"rate limit", output.ErrRateLimit(30), true},
		{"server error", output.ErrAPI(500, "down"), true},
		{"api 500", &registry.APIError{StatusCode: 500, Msg: "down"}, true},
		{"api 429", &registry.APIError{StatusCode: 429, Msg: "rate"}, true},
		{"api 403", &registry.APIError{StatusCode: 403, Msg: "no"}, false},
		{"transport error", fmt.Errorf("publish request: %w", fmt.Errorf("dial tcp: lookup example.com: no such host")), true},
		{"forbidden", output.ErrForbidden("no"), false},
		{"not found", output.ErrNotFound("pkg", "x"), false},
		{"context cancelled", context.Canceled, false},
		{"generic error", io.ErrUnexpectedEOF, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- helpers ---

func writeTestManifest(t *testing.T, dir, fullName, version string) {
	t.Helper()
	_, name := manifest.ParseFullName(fullName)
	scope := strings.TrimPrefix(fullName, "@")
	scope = strings.SplitN(scope, "/", 2)[0]

	m := manifest.Scaffold(manifest.TypeSkill, scope, name)
	m.Version = version
	m.Description = "Test skill"
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// loadTestManifest loads the manifest from a test directory (for stageAndArchive calls).
func loadTestManifest(t *testing.T, dir string) *manifest.Manifest {
	t.Helper()
	m, err := manifest.LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
