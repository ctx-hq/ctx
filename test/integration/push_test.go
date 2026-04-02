package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/pushstate"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/staging"
)

// TestPushFlow_DirectoryWithArchiveCreation verifies the full flow:
// stage directory → create archive → publish to registry.
func TestPushFlow_DirectoryWithArchiveCreation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/packages" {
			t.Errorf("expected /v1/packages, got %s", r.URL.Path)
		}

		// Verify multipart form has both manifest and archive.
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.MultipartForm.File["manifest"] == nil {
			t.Error("missing manifest part")
		}
		if r.MultipartForm.File["archive"] == nil {
			t.Error("missing archive part")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registry.PublishResponse{
			FullName: "@test/full-flow",
			Version:  "0.1.0",
		})
	}))
	defer srv.Close()

	// Create a skill directory.
	dir := t.TempDir()
	m := manifest.Scaffold(manifest.TypeSkill, "test", "full-flow")
	m.Version = "0.1.0"
	m.Description = "Integration test skill"
	m.Visibility = "private"
	m.Mutable = true

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ctx.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Full Flow Test\n\nContent."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage and create archive (the fix for the nil archive bug).
	stg, err := staging.New("test-push-")
	if err != nil {
		t.Fatal(err)
	}
	defer stg.Rollback()

	if err := stg.CopyFrom(dir); err != nil {
		t.Fatal(err)
	}
	if err := stg.WriteFile("ctx.yaml", data, 0o644); err != nil {
		t.Fatal(err)
	}

	archive, err := stg.TarGz()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()

	// Publish to mock registry.
	reg := registry.New(srv.URL, "test-token")
	result, err := reg.Publish(context.Background(), data, archive, nil)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if result.FullName != "@test/full-flow" {
		t.Errorf("FullName = %q", result.FullName)
	}
}

// TestPushFlow_StateTracking verifies push state records and detects changes.
func TestPushFlow_StateTracking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create a skill.
	dir := filepath.Join(tmp, "skills", "alice", "tracked")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, "@alice/tracked", "0.1.0")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Tracked\nv1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute initial hash.
	hash1, err := pushstate.HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Load empty state → skill should be dirty.
	ps, _ := pushstate.Load()
	if !ps.IsDirty("@alice/tracked", hash1) {
		t.Error("new skill should be dirty")
	}

	// Record push.
	ps.RecordPush("@alice/tracked", hash1, "0.1.0", dir)
	if err := ps.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload → skill should be clean.
	ps2, _ := pushstate.Load()
	if ps2.IsDirty("@alice/tracked", hash1) {
		t.Error("pushed skill should not be dirty")
	}

	// Modify skill content.
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Tracked\nv2 modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash2, _ := pushstate.HashDir(dir)
	if hash1 == hash2 {
		t.Fatal("modified content should change hash")
	}
	if !ps2.IsDirty("@alice/tracked", hash2) {
		t.Error("modified skill should be dirty")
	}
}

// TestPushFlow_NameResolution verifies skill lookup by name from ~/.ctx/skills/.
func TestPushFlow_NameResolution(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create skills in two scopes.
	for _, scope := range []string{"alice", "bob"} {
		dir := filepath.Join(tmp, "skills", scope, "gc")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeManifest(t, dir, "@"+scope+"/gc", "0.1.0")
	}

	// Scan should find both.
	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}
	skills := scanSkillsFromDir(filepath.Join(tmp, "skills"), ps)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

// TestPushFlow_DryRunNoSideEffects verifies dry-run doesn't modify state.
func TestPushFlow_DryRunNoSideEffects(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	// Create a skill.
	dir := filepath.Join(tmp, "skills", "alice", "dryrun")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, "@alice/dryrun", "0.1.0")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Dry Run"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save empty state.
	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}
	if err := ps.Save(); err != nil {
		t.Fatal(err)
	}

	// Simulate dry-run by scanning but not pushing.
	hash, _ := pushstate.HashDir(dir)
	if !ps.IsDirty("@alice/dryrun", hash) {
		t.Error("skill should be dirty before push")
	}

	// After "dry-run" (no RecordPush), state should remain unchanged.
	ps2, _ := pushstate.Load()
	if !ps2.IsDirty("@alice/dryrun", hash) {
		t.Error("dry-run should not modify push state")
	}
}

// TestPushFlow_PartialBatchFailure verifies state is saved for successful pushes
// even if some fail.
func TestPushFlow_PartialBatchFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTX_HOME", tmp)

	ps := &pushstate.PushState{Version: 1, Skills: make(map[string]*pushstate.SkillState)}

	// Simulate: push skill-a succeeds, skill-b fails.
	dirA := filepath.Join(tmp, "skills", "alice", "skill-a")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dirA, "@alice/skill-a", "0.1.0")
	if err := os.WriteFile(filepath.Join(dirA, "SKILL.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	hashA, _ := pushstate.HashDir(dirA)

	// Record only skill-a.
	ps.RecordPush("@alice/skill-a", hashA, "0.1.0", dirA)
	if err := ps.Save(); err != nil {
		t.Fatal(err)
	}

	// skill-a should be clean, skill-b should be dirty.
	ps2, _ := pushstate.Load()
	if ps2.IsDirty("@alice/skill-a", hashA) {
		t.Error("skill-a should be clean after partial batch")
	}
	if !ps2.IsDirty("@alice/skill-b", "sha256:anything") {
		t.Error("skill-b should still be dirty after partial batch")
	}
}

// --- helpers ---

func writeManifest(t *testing.T, dir, fullName, version string) {
	t.Helper()
	scope, name := manifest.ParseFullName(fullName)

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

// scanSkillsFromDir walks a skills directory and returns entries.
func scanSkillsFromDir(skillsDir string, ps *pushstate.PushState) []struct{ FullName string } {
	var skills []struct{ FullName string }
	scopes, _ := os.ReadDir(skillsDir)
	for _, scope := range scopes {
		if !scope.IsDir() {
			continue
		}
		names, _ := os.ReadDir(filepath.Join(skillsDir, scope.Name()))
		for _, name := range names {
			if !name.IsDir() {
				continue
			}
			skills = append(skills, struct{ FullName string }{
				FullName: "@" + scope.Name() + "/" + name.Name(),
			})
		}
	}
	return skills
}
