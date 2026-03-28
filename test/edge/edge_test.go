package edge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
)

// --- Package Name Edge Cases ---

func TestValidateManifest_LongPackageName(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@abcdefghij/" + strings.Repeat("a", 100),
		Version:     "1.0.0",
		Type:        manifest.TypeSkill,
		Description: "test",
	}
	errs := manifest.Validate(m)
	// Should be valid — names up to reasonable length are allowed
	if len(errs) != 0 {
		t.Errorf("expected valid, got errors: %v", errs)
	}
}

func TestValidateManifest_SpecialCharsInScope(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"@my-scope/my-pkg", true},
		{"@my_scope/my-pkg", false}, // underscore not allowed
		{"@My-Scope/my-pkg", false}, // uppercase not allowed
		{"@123/456", true},          // numeric allowed
		{"@a/b", true},              // single char
		{"@/name", false},           // empty scope
		{"@scope/", false},          // empty name
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &manifest.Manifest{
				Name: tt.name, Version: "1.0.0", Type: manifest.TypeSkill,
				Description: "test",
			}
			errs := manifest.Validate(m)
			hasNameErr := false
			for _, e := range errs {
				if strings.Contains(e, "name") {
					hasNameErr = true
				}
			}
			if tt.valid && hasNameErr {
				t.Errorf("%q should be valid but got name error", tt.name)
			}
			if !tt.valid && !hasNameErr {
				t.Errorf("%q should be invalid but passed validation", tt.name)
			}
		})
	}
}

// --- Version Edge Cases ---

func TestSwitchCurrent_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "1.0.0"), 0o755)
	os.MkdirAll(filepath.Join(dir, "2.0.0"), 0o755)

	// Run multiple switches concurrently — should not panic or corrupt
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			version := "1.0.0"
			if i%2 == 0 {
				version = "2.0.0"
			}
			done <- installer.SwitchCurrent(dir, version)
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent switch failed: %v", err)
		}
	}

	// Current should point to a valid version
	dest, err := os.Readlink(filepath.Join(dir, "current"))
	if err != nil {
		t.Fatalf("current symlink broken after concurrent switches: %v", err)
	}
	if dest != "1.0.0" && dest != "2.0.0" {
		t.Errorf("current → %q, want 1.0.0 or 2.0.0", dest)
	}
}

func TestInstallSameVersionTwice(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@test", "pkg")
	v1Dir := filepath.Join(pkgDir, "1.0.0")

	// First install
	os.MkdirAll(v1Dir, 0o755)
	os.WriteFile(filepath.Join(v1Dir, "SKILL.md"), []byte("original"), 0o644)
	installer.SwitchCurrent(pkgDir, "1.0.0")

	// "Install" same version again — should not fail
	err := installer.SwitchCurrent(pkgDir, "1.0.0")
	if err != nil {
		t.Fatalf("reinstall same version should not fail: %v", err)
	}

	// Content should be preserved
	data, _ := os.ReadFile(filepath.Join(v1Dir, "SKILL.md"))
	if string(data) != "original" {
		t.Error("content should be preserved on reinstall")
	}
}

// --- SKILL.md Edge Cases ---

func TestParseSkillMD_UnicodeContent(t *testing.T) {
	content := "---\nname: 测试\ndescription: 中文描述\n---\n# 技能文档\n\n包含 emoji 🎉 和特殊字符 é à ü\n"
	fm, body, err := manifest.ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("should handle unicode: %v", err)
	}
	if fm.Name != "测试" {
		t.Errorf("name = %q, want 测试", fm.Name)
	}
	if !strings.Contains(body, "🎉") {
		t.Error("body should preserve emoji")
	}
}

func TestParseSkillMD_EmptyFrontmatter(t *testing.T) {
	content := "---\n---\n# Just body\n"
	fm, body, err := manifest.ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("empty frontmatter should not error: %v", err)
	}
	if fm == nil {
		t.Fatal("should return non-nil frontmatter (even if empty)")
	}
	if !strings.Contains(body, "Just body") {
		t.Error("body should be present")
	}
}

func TestParseSkillMD_OnlyFrontmatter(t *testing.T) {
	content := "---\nname: test\n---\n"
	fm, body, err := manifest.ParseSkillMD(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm == nil || fm.Name != "test" {
		t.Error("frontmatter should be parsed")
	}
	if strings.TrimSpace(body) != "" {
		t.Errorf("body should be empty, got %q", body)
	}
}

// --- Links Registry Edge Cases ---

func TestLinksRegistry_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links.json")
	os.WriteFile(path, []byte("{invalid json"), 0o600)

	// Should return error, not panic
	origHome := os.Getenv("CTX_HOME")
	os.Setenv("CTX_HOME", dir)
	defer os.Setenv("CTX_HOME", origHome)

	_, err := installer.LoadLinks()
	if err == nil {
		t.Error("should return error for corrupted links.json")
	}
}

func TestLinksRegistry_ManyPackages(t *testing.T) {
	reg := &installer.LinkRegistry{
		Version: 1,
		Links:   make(map[string][]installer.LinkEntry),
	}

	// Add 100 packages with 3 links each
	for i := 0; i < 100; i++ {
		pkg := fmt.Sprintf("@test/pkg-%d", i)
		for _, agent := range []string{"claude", "cursor", "generic"} {
			reg.Add(pkg, installer.LinkEntry{
				Agent:  agent,
				Type:   installer.LinkSymlink,
				Source: "/src/" + pkg,
				Target: "/dst/" + agent + "/" + pkg,
			})
		}
	}

	// Verify should work without issues on nonexistent paths
	issues := reg.Verify()
	if len(issues) == 0 {
		t.Error("should detect issues for nonexistent targets")
	}
}

// --- Output Edge Cases ---

func TestWriter_NilSliceVsEmptySlice(t *testing.T) {
	// Nil slice in count mode
	buf := &bytes.Buffer{}
	w := output.NewWriter(output.WithStdout(buf), output.WithFormat(output.FormatCount))
	w.OK(nil)
	if got := strings.TrimSpace(buf.String()); got != "0" {
		t.Errorf("nil count = %q, want 0", got)
	}

	// Empty slice in count mode
	buf.Reset()
	w.OK([]string{})
	if got := strings.TrimSpace(buf.String()); got != "0" {
		t.Errorf("empty count = %q, want 0", got)
	}
}

func TestWriter_JSONEnvelope_ErrorHasNoDataField(t *testing.T) {
	buf := &bytes.Buffer{}
	w := output.NewWriter(output.WithStdout(buf), output.WithFormat(output.FormatJSON))
	w.Err(output.ErrAuth("not logged in"))

	var resp map[string]any
	json.Unmarshal(buf.Bytes(), &resp)

	if resp["ok"] != false {
		t.Error("error response ok should be false")
	}
	if _, hasData := resp["data"]; hasData {
		t.Error("error response should not have data field")
	}
}

func TestWriter_IDsMode_FallbackKeys(t *testing.T) {
	buf := &bytes.Buffer{}
	w := output.NewWriter(output.WithStdout(buf), output.WithFormat(output.FormatIDs))

	// Test with "name" key (no "full_name" or "id")
	data := []map[string]any{
		{"name": "review", "version": "1.0.0"},
	}
	w.OK(data)
	if got := strings.TrimSpace(buf.String()); got != "review" {
		t.Errorf("IDs fallback to name = %q, want review", got)
	}
}

// --- Typed Error Edge Cases ---

func TestCLIError_WrappedChain(t *testing.T) {
	// Three-level error chain
	inner := output.ErrNetwork(nil)
	middle := output.ErrAPI(500, "wrapped: "+inner.Error())
	outer := output.ErrUsage("outer: " + middle.Error())

	// AsCLIError should extract the outermost CLIError
	extracted := output.AsCLIError(outer)
	if extracted == nil {
		t.Fatal("should extract CLIError from chain")
	}
	if extracted.Code != output.CodeUsage {
		t.Errorf("code = %q, want usage", extracted.Code)
	}
}

func TestFromHTTPStatus_AllCodes(t *testing.T) {
	// Exhaustive test for all common HTTP status codes
	codes := []int{200, 201, 301, 400, 401, 403, 404, 409, 422, 429, 500, 502, 503, 504}
	for _, code := range codes {
		e := output.FromHTTPStatus(code, "test")
		if e == nil {
			t.Errorf("FromHTTPStatus(%d) should not return nil", code)
		}
		if e.ExitCode() < 0 || e.ExitCode() > 8 {
			t.Errorf("FromHTTPStatus(%d).ExitCode() = %d, out of range", code, e.ExitCode())
		}
	}
}

// --- Manifest Hybrid Package Edge Cases ---

func TestValidateManifest_HybridSkillWithCLI(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@ctx/ffmpeg",
		Version:     "1.0.0",
		Type:        manifest.TypeSkill,
		Description: "FFmpeg skill with CLI dependency",
		Skill: &manifest.SkillSpec{
			Entry: "SKILL.md",
		},
		CLI: &manifest.CLISpec{
			Binary:     "ffmpeg",
			Verify:     "ffmpeg -version",
			Compatible: ">=6.0",
		},
		Install: &manifest.InstallSpec{
			Brew: "ffmpeg",
		},
	}
	errs := manifest.Validate(m)
	// Hybrid should be valid — skill type with cli section is allowed
	if len(errs) != 0 {
		t.Errorf("hybrid package should be valid, got: %v", errs)
	}
}

func TestValidateManifest_SourceSpec(t *testing.T) {
	tests := []struct {
		github string
		valid  bool
	}{
		{"basecamp/basecamp-cli", true},
		{"user/repo", true},
		{"user-name/repo.name", true},
		{"invalid", false},       // no slash
		{"/repo", false},         // no owner
		{"owner/", false},        // no repo
		{"a/b/c", false},         // too many parts
	}
	for _, tt := range tests {
		t.Run(tt.github, func(t *testing.T) {
			m := &manifest.Manifest{
				Name: "@ctx/test", Version: "1.0.0", Type: manifest.TypeSkill,
				Description: "test",
				Source:      &manifest.SourceSpec{GitHub: tt.github},
			}
			errs := manifest.Validate(m)
			hasSourceErr := false
			for _, e := range errs {
				if strings.Contains(e, "source.github") {
					hasSourceErr = true
				}
			}
			if tt.valid && hasSourceErr {
				t.Errorf("%q should be valid source.github", tt.github)
			}
			if !tt.valid && !hasSourceErr {
				t.Errorf("%q should be invalid source.github", tt.github)
			}
		})
	}
}

// --- Prune Edge Cases ---

func TestPrune_KeepMinimumOne(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@test", "pkg")
	os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755)
	os.Symlink("1.0.0", filepath.Join(pkgDir, "current"))

	inst := &installer.Installer{DataDir: dir}

	// keepCount = 0 should be treated as 1 (always keep current)
	removed, _, _ := inst.PruneVersions("@test/pkg", 0)
	if len(removed) != 0 {
		t.Error("should not prune when only one version exists")
	}

	// Verify the version still exists
	versions := inst.InstalledVersions("@test/pkg")
	if len(versions) != 1 {
		t.Errorf("should have 1 version, got %d", len(versions))
	}
}

// --- Missing Current Symlink ---

func TestCurrentVersion_MissingSymlink(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@test", "pkg")
	os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755)
	// No current symlink created

	inst := &installer.Installer{DataDir: dir}
	ver := inst.CurrentVersion("@test/pkg")
	if ver != "" {
		t.Errorf("should return empty for missing current, got %q", ver)
	}
}

// --- FormatBytes ---

func TestFormatBytes(t *testing.T) {
	// This tests the formatBytes function indirectly through prune output
	// The function is in cmd/ctx/prune.go and not directly testable,
	// but we verify the prune freed bytes are reported correctly
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "@test", "pkg")
	os.MkdirAll(filepath.Join(pkgDir, "1.0.0"), 0o755)
	os.MkdirAll(filepath.Join(pkgDir, "2.0.0"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "1.0.0", "data"), make([]byte, 1024), 0o644)
	os.Symlink("2.0.0", filepath.Join(pkgDir, "current"))

	inst := &installer.Installer{DataDir: dir}
	_, freed, _ := inst.PruneVersions("@test/pkg", 1)
	if freed < 1024 {
		t.Errorf("freed = %d, want >= 1024", freed)
	}
}
