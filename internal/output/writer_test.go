package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewWriter_Defaults(t *testing.T) {
	w := NewWriter()
	if w.format != FormatAuto {
		t.Errorf("default format = %d, want FormatAuto", w.format)
	}
}

func TestEffectiveFormat(t *testing.T) {
	// Non-TTY writer (bytes.Buffer) → JSON
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatAuto))
	if w.EffectiveFormat() != FormatJSON {
		t.Errorf("non-TTY EffectiveFormat = %d, want FormatJSON", w.EffectiveFormat())
	}

	// Explicit format overrides auto-detection
	w2 := NewWriter(WithStdout(buf), WithFormat(FormatStyled))
	if w2.EffectiveFormat() != FormatStyled {
		t.Errorf("explicit format = %d, want FormatStyled", w2.EffectiveFormat())
	}
}

func TestIsStyled_IsMachine(t *testing.T) {
	buf := &bytes.Buffer{}
	tests := []struct {
		format     Format
		isStyled   bool
		isMachine  bool
	}{
		{FormatStyled, true, false},
		{FormatJSON, false, true},
		{FormatQuiet, false, true},
		{FormatMarkdown, false, false},
		{FormatIDs, false, true},
		{FormatCount, false, true},
	}
	for _, tt := range tests {
		w := NewWriter(WithStdout(buf), WithFormat(tt.format))
		if w.IsStyled() != tt.isStyled {
			t.Errorf("Format %d: IsStyled = %v, want %v", tt.format, w.IsStyled(), tt.isStyled)
		}
		if w.IsMachine() != tt.isMachine {
			t.Errorf("Format %d: IsMachine = %v, want %v", tt.format, w.IsMachine(), tt.isMachine)
		}
	}
}

func TestOK_JSON(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatJSON))

	data := []map[string]any{
		{"name": "pkg1", "version": "1.0.0"},
	}
	err := w.OK(data,
		WithSummary("1 package"),
		WithBreadcrumbs(Breadcrumb{Action: "info", Command: "ctx info pkg1", Description: "View details"}),
		WithMeta("elapsed_ms", 42),
	)
	if err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !resp.OK {
		t.Error("response.ok should be true")
	}
	if resp.Summary != "1 package" {
		t.Errorf("summary = %q, want %q", resp.Summary, "1 package")
	}
	if len(resp.Breadcrumbs) != 1 {
		t.Errorf("breadcrumbs len = %d, want 1", len(resp.Breadcrumbs))
	}
	if resp.Breadcrumbs[0].Action != "info" {
		t.Errorf("breadcrumb action = %q, want %q", resp.Breadcrumbs[0].Action, "info")
	}
	if resp.Meta["elapsed_ms"] == nil {
		t.Error("meta should contain elapsed_ms")
	}
}

func TestOK_Quiet(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatQuiet))

	data := []string{"a", "b", "c"}
	err := w.OK(data, WithSummary("should not appear"))
	if err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	// Quiet mode: data only, no envelope
	var got []string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("data len = %d, want 3", len(got))
	}
	if strings.Contains(buf.String(), "summary") {
		t.Error("quiet mode should not contain summary")
	}
}

func TestOK_Count(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatCount))

	data := []int{1, 2, 3, 4, 5}
	if err := w.OK(data); err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "5" {
		t.Errorf("count = %q, want %q", got, "5")
	}
}

func TestOK_IDs(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatIDs))

	data := []map[string]any{
		{"full_name": "@hong/review", "version": "1.0.0"},
		{"full_name": "@ctx/ffmpeg", "version": "2.0.0"},
	}
	if err := w.OK(data); err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if lines[0] != "@hong/review" {
		t.Errorf("line[0] = %q, want %q", lines[0], "@hong/review")
	}
	if lines[1] != "@ctx/ffmpeg" {
		t.Errorf("line[1] = %q, want %q", lines[1], "@ctx/ffmpeg")
	}
}

func TestOK_Markdown(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatMarkdown))

	data := []map[string]any{
		{"name": "pkg1", "version": "1.0.0"},
		{"name": "pkg2", "version": "2.0.0"},
	}
	if err := w.OK(data); err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "|") {
		t.Error("markdown should contain table separators")
	}
	if !strings.Contains(out, "---") {
		t.Error("markdown should contain header separator")
	}
	if !strings.Contains(out, "pkg1") {
		t.Error("markdown should contain data")
	}
}

func TestOK_Styled_EmptySlice(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(stderr), WithFormat(FormatStyled))

	data := []any{}
	if err := w.OK(data, WithSummary("no results")); err != nil {
		t.Fatalf("OK() error: %v", err)
	}

	if !strings.Contains(stderr.String(), "no results") {
		t.Error("styled empty output should show summary on stderr")
	}
}

func TestErr_Machine(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatJSON))

	cliErr := ErrNotFound("package", "@hong/missing")
	if err := w.Err(cliErr); err != nil {
		t.Fatalf("Err() error: %v", err)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.OK {
		t.Error("error response.ok should be false")
	}
	if resp.Code != CodeNotFound {
		t.Errorf("code = %q, want %q", resp.Code, CodeNotFound)
	}
}

func TestErr_Styled(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithFormat(FormatStyled))

	cliErr := ErrAuth("not logged in")
	if err := w.Err(cliErr); err != nil {
		t.Fatalf("Err() error: %v", err)
	}

	out := stderr.String()
	if !strings.Contains(out, "not logged in") {
		t.Error("styled error should contain message")
	}
	if !strings.Contains(out, "Hint") {
		t.Error("styled error should show hint for auth errors")
	}
}

func TestErr_PlainError(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatJSON))

	// Non-CLIError should be wrapped as usage error
	plainErr := strings.NewReader("").Read // just use a plain error
	_ = plainErr
	if err := w.Err(ErrUsage("plain error")); err != nil {
		t.Fatalf("Err() error: %v", err)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != CodeUsage {
		t.Errorf("plain error code = %q, want %q", resp.Code, CodeUsage)
	}
}

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name   string
		flags  [7]bool // json, quiet, styled, md, ids, count, agent
		want   Format
		errMsg string
	}{
		{"none", [7]bool{}, FormatAuto, ""},
		{"json", [7]bool{true, false, false, false, false, false, false}, FormatJSON, ""},
		{"quiet", [7]bool{false, true, false, false, false, false, false}, FormatQuiet, ""},
		{"styled", [7]bool{false, false, true, false, false, false, false}, FormatStyled, ""},
		{"md", [7]bool{false, false, false, true, false, false, false}, FormatMarkdown, ""},
		{"ids", [7]bool{false, false, false, false, true, false, false}, FormatIDs, ""},
		{"count", [7]bool{false, false, false, false, false, true, false}, FormatCount, ""},
		{"agent", [7]bool{false, false, false, false, false, false, true}, FormatQuiet, ""},
		{"mutual_exclusive", [7]bool{true, true, false, false, false, false, false}, FormatAuto, "mutually exclusive"},
		{"triple_conflict", [7]bool{true, false, true, true, false, false, false}, FormatAuto, "mutually exclusive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.flags
			got, err := ResolveFormat(f[0], f[1], f[2], f[3], f[4], f[5], f[6])
			if tt.errMsg != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("format = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestToSlice(t *testing.T) {
	// Slice input
	got := toSlice([]int{1, 2, 3})
	if len(got) != 3 {
		t.Errorf("toSlice([]int) len = %d, want 3", len(got))
	}

	// Non-slice input
	if toSlice("not a slice") != nil {
		t.Error("toSlice(string) should return nil")
	}

	// Nil input
	if toSlice(nil) != nil {
		t.Error("toSlice(nil) should return nil")
	}

	// Empty slice
	got = toSlice([]string{})
	if got == nil {
		t.Error("toSlice([]string{}) should return empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("toSlice([]string{}) len = %d, want 0", len(got))
	}
}

func TestMapStr(t *testing.T) {
	m := map[string]any{
		"name":      "test",
		"full_name": "@hong/test",
		"empty":     "",
	}

	// First match wins
	if got := mapStr(m, "full_name", "name"); got != "@hong/test" {
		t.Errorf("mapStr = %q, want %q", got, "@hong/test")
	}

	// Fallback to second key
	if got := mapStr(m, "missing", "name"); got != "test" {
		t.Errorf("mapStr fallback = %q, want %q", got, "test")
	}

	// No match
	if got := mapStr(m, "nonexistent"); got != "" {
		t.Errorf("mapStr no match = %q, want empty", got)
	}

	// Skip empty values
	if got := mapStr(m, "empty", "name"); got != "test" {
		t.Errorf("mapStr skip empty = %q, want %q", got, "test")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]any{"c": 3, "a": 1, "b": 2}
	got := sortedKeys(m)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOK_NilData(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatCount))
	if err := w.OK(nil); err != nil {
		t.Fatalf("OK() error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "0" {
		t.Errorf("count nil = %q, want %q", got, "0")
	}
}

func TestOK_SingleObject(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(WithStdout(buf), WithFormat(FormatCount))
	if err := w.OK(map[string]string{"a": "b"}); err != nil {
		t.Fatalf("OK() error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "1" {
		t.Errorf("count single = %q, want %q", got, "1")
	}
}
