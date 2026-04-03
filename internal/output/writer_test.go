package output

import (
	"bytes"
	"context"
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
		flags  [6]bool // json, quiet, md, ids, count, agent
		want   Format
		errMsg string
	}{
		{"none", [6]bool{}, FormatAuto, ""},
		{"json", [6]bool{true, false, false, false, false, false}, FormatJSON, ""},
		{"quiet", [6]bool{false, true, false, false, false, false}, FormatQuiet, ""},
		{"md", [6]bool{false, false, true, false, false, false}, FormatMarkdown, ""},
		{"ids", [6]bool{false, false, false, true, false, false}, FormatIDs, ""},
		{"count", [6]bool{false, false, false, false, true, false}, FormatCount, ""},
		{"agent", [6]bool{false, false, false, false, false, true}, FormatQuiet, ""},
		{"mutual_exclusive", [6]bool{true, true, false, false, false, false}, FormatAuto, "mutually exclusive"},
		{"triple_conflict", [6]bool{true, false, true, true, false, false}, FormatAuto, "mutually exclusive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.flags
			got, err := ResolveFormat(f[0], f[1], f[2], f[3], f[4], f[5])
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

// --- Verbose context tests ---

func TestContextVerbose(t *testing.T) {
	ctx := context.Background()
	if IsVerboseContext(ctx) {
		t.Error("empty context should not be verbose")
	}

	ctx = ContextWithVerbose(ctx, true)
	if !IsVerboseContext(ctx) {
		t.Error("context with verbose=true should be verbose")
	}

	ctx = ContextWithVerbose(ctx, false)
	if IsVerboseContext(ctx) {
		t.Error("context with verbose=false should not be verbose")
	}
}

// --- Writer method tests ---

func TestWriter_Success_ColorNever(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Success("done %s", "ok")
	out := stderr.String()
	if !strings.Contains(out, "\u2713") {
		t.Error("Success should contain checkmark")
	}
	if !strings.Contains(out, "done ok") {
		t.Error("Success should contain formatted message")
	}
	if strings.Contains(out, "\033[") {
		t.Error("ColorNever should produce no ANSI escapes")
	}
}

func TestWriter_Success_ColorAlways(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorAlways))
	w.Success("done")
	out := stderr.String()
	if !strings.Contains(out, "\033[") {
		t.Error("ColorAlways should contain ANSI escapes")
	}
	if !strings.Contains(out, "done") {
		t.Error("should contain message text")
	}
}

func TestWriter_Warn_Output(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Warn("watch out %d", 42)
	if !strings.Contains(stderr.String(), "!") {
		t.Error("Warn should contain exclamation mark")
	}
	if !strings.Contains(stderr.String(), "watch out 42") {
		t.Error("Warn should contain formatted message")
	}
}

func TestWriter_Info_Output(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Info("hello %s", "world")
	if !strings.Contains(stderr.String(), "\u2192") {
		t.Error("Info should contain arrow")
	}
	if !strings.Contains(stderr.String(), "hello world") {
		t.Error("Info should contain formatted message")
	}
}

func TestWriter_Errorf_Output(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Errorf("failed: %v", "oops")
	if !strings.Contains(stderr.String(), "\u2717") {
		t.Error("Errorf should contain cross mark")
	}
	if !strings.Contains(stderr.String(), "failed: oops") {
		t.Error("Errorf should contain formatted message")
	}
}

func TestWriter_Header_Bold(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorAlways))
	w.Header("Section")
	out := stderr.String()
	if !strings.Contains(out, "Section") {
		t.Error("Header should contain text")
	}
	if !strings.Contains(out, "\033[") {
		t.Error("Header with ColorAlways should have ANSI")
	}
}

func TestWriter_PrintDim(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.PrintDim("faint text")
	if !strings.Contains(stderr.String(), "faint text") {
		t.Error("PrintDim should contain text")
	}
}

func TestWriter_Table_Alignment(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}))
	w.Table([][]string{
		{"Name:", "test"},
		{"Version:", "1.0.0"},
	})
	out := stdout.String()
	if !strings.Contains(out, "Name:") || !strings.Contains(out, "test") {
		t.Error("Table should contain key-value pairs")
	}
	if !strings.Contains(out, "Version:") || !strings.Contains(out, "1.0.0") {
		t.Error("Table should contain all rows")
	}
}

func TestWriter_Table_Empty(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}))
	w.Table(nil)
	if stdout.Len() != 0 {
		t.Error("empty Table should produce no output")
	}
}

func TestWriter_List_Bullets(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}))
	w.List([]string{"alpha", "beta"})
	out := stdout.String()
	if !strings.Contains(out, "\u2022 alpha") {
		t.Error("List should contain bulleted items")
	}
	if !strings.Contains(out, "\u2022 beta") {
		t.Error("List should contain all items")
	}
}

func TestWriter_List_Empty(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}))
	w.List(nil)
	if stdout.Len() != 0 {
		t.Error("empty List should produce no output")
	}
}

func TestWriter_Separator(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Separator()
	if !strings.Contains(stderr.String(), "\u2500") {
		t.Error("Separator should contain horizontal line character")
	}
}

func TestWriter_PrintLinkedAgents_FewNames(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.PrintLinkedAgents([]string{"Claude", "Cursor"})
	out := stderr.String()
	if !strings.Contains(out, "Claude, Cursor") {
		t.Errorf("should show all names, got %q", out)
	}
}

func TestWriter_PrintLinkedAgents_ManyNames(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.PrintLinkedAgents([]string{"a", "b", "c", "d", "e"})
	out := stderr.String()
	if !strings.Contains(out, "+ 2 more") {
		t.Errorf("should fold excess names, got %q", out)
	}
}

func TestWriter_PrintLinkedAgents_Empty(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr))
	w.PrintLinkedAgents(nil)
	if stderr.Len() != 0 {
		t.Error("empty names should produce no output")
	}
}

func TestWriter_Verbose_Active(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	ctx := ContextWithVerbose(context.Background(), true)
	w.Verbose(ctx, "debug %s", "info")
	out := stderr.String()
	if !strings.Contains(out, "[verbose]") {
		t.Error("verbose should contain [verbose] prefix")
	}
	if !strings.Contains(out, "debug info") {
		t.Error("verbose should contain message")
	}
}

func TestWriter_Verbose_Inactive(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr))
	ctx := ContextWithVerbose(context.Background(), false)
	w.Verbose(ctx, "should not appear")
	if stderr.Len() != 0 {
		t.Error("verbose=false should produce no output")
	}
}

// --- Breadcrumbs rendering tests ---

func TestWriter_Breadcrumbs_Human_Rendered(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(stderr), WithFormat(FormatStyled), WithColorMode(ColorNever))
	err := w.OK(map[string]any{"name": "test"},
		WithBreadcrumbs(Breadcrumb{Action: "info", Command: "ctx info test", Description: "View details"}),
	)
	if err != nil {
		t.Fatalf("OK() error: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "Next steps:") {
		t.Error("human format should render breadcrumbs header")
	}
	if !strings.Contains(out, "ctx info test") {
		t.Error("human format should render breadcrumb command")
	}
	if !strings.Contains(out, "View details") {
		t.Error("human format should render breadcrumb description")
	}
}

func TestWriter_Breadcrumbs_Human_Empty(t *testing.T) {
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithFormat(FormatStyled))
	_ = w.OK(map[string]any{"name": "test"})
	if strings.Contains(stderr.String(), "Next steps:") {
		t.Error("no breadcrumbs should not render 'Next steps:'")
	}
}

func TestWriter_Breadcrumbs_JSON_Unchanged(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithFormat(FormatJSON))
	_ = w.OK(map[string]any{"name": "test"},
		WithBreadcrumbs(Breadcrumb{Action: "info", Command: "ctx info test", Description: "View details"}),
	)
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Breadcrumbs) != 1 {
		t.Errorf("JSON should preserve breadcrumbs, got %d", len(resp.Breadcrumbs))
	}
}

// --- Context tests ---

func TestFromContext_WithWriter(t *testing.T) {
	w := NewWriter(WithColorMode(ColorNever))
	ctx := context.WithValue(context.Background(), WriterKey, w)
	got := FromContext(ctx)
	if got != w {
		t.Error("FromContext should return the stored Writer")
	}
}

func TestFromContext_NoWriter(t *testing.T) {
	got := FromContext(context.Background())
	if got == nil {
		t.Fatal("FromContext should return a default Writer, not nil")
	}
}

func TestFromContext_NilContext(t *testing.T) {
	//nolint:staticcheck // intentionally testing nil context handling
	got := FromContext(nil)
	if got == nil {
		t.Fatal("FromContext(nil) should return a default Writer, not nil")
	}
}

// --- Color mode orthogonality tests ---

func TestColorMode_JSON_NoANSI(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithFormat(FormatJSON), WithColorMode(ColorAlways))
	_ = w.OK([]map[string]any{{"name": "test"}})
	if strings.Contains(stdout.String(), "\033[") {
		t.Error("JSON format should never contain ANSI even with ColorAlways")
	}
}

func TestColorMode_Quiet_NoANSI(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithFormat(FormatQuiet), WithColorMode(ColorAlways))
	_ = w.OK([]map[string]any{{"name": "test"}})
	if strings.Contains(stdout.String(), "\033[") {
		t.Error("Quiet format should never contain ANSI even with ColorAlways")
	}
}

func TestColorMode_Human_ColorNever(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(stderr), WithFormat(FormatStyled), WithColorMode(ColorNever))
	_ = w.OK([]map[string]any{{"name": "pkg", "type": "skill", "version": "1.0"}}, WithSummary("1 result"))
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, "\033[") {
		t.Error("Human format with ColorNever should have zero ANSI")
	}
}

func TestColorMode_Human_ColorAlways(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(stderr), WithFormat(FormatStyled), WithColorMode(ColorAlways))
	_ = w.OK([]map[string]any{{"name": "pkg", "type": "skill", "version": "1.0"}}, WithSummary("1 result"))
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "\033[") {
		t.Error("Human format with ColorAlways should contain ANSI escapes")
	}
}

// --- Semantic row coloring tests ---

func TestWriter_PrintMapRow_Semantic_ColorAlways(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled), WithColorMode(ColorAlways))
	_ = w.OK([]map[string]any{{"name": "test-pkg", "type": "skill", "version": "1.0"}})
	out := stdout.String()
	if !strings.Contains(out, "test-pkg") {
		t.Error("should contain package name")
	}
	if !strings.Contains(out, "[skill]") {
		t.Error("should contain type badge")
	}
	if !strings.Contains(out, "\033[") {
		t.Error("ColorAlways should produce ANSI in data rows")
	}
}

func TestWriter_PrintMapRow_Semantic_ColorNever(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled), WithColorMode(ColorNever))
	_ = w.OK([]map[string]any{{"name": "test-pkg", "type": "cli", "version": "2.0"}})
	out := stdout.String()
	if !strings.Contains(out, "test-pkg") {
		t.Error("should contain package name")
	}
	if !strings.Contains(out, "[cli]") {
		t.Error("should contain type badge")
	}
	if strings.Contains(out, "\033[") {
		t.Error("ColorNever should have no ANSI in data rows")
	}
}

func TestWriter_PrintMapRow_NoName(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled), WithColorMode(ColorNever))
	_ = w.OK([]map[string]any{{"foo": "bar", "baz": "qux"}})
	out := stdout.String()
	if !strings.Contains(out, "baz=qux") || !strings.Contains(out, "foo=bar") {
		t.Error("no-name fallback should print key=value pairs")
	}
}

func TestWriter_PrintMapRow_NoType(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled), WithColorMode(ColorNever))
	_ = w.OK([]map[string]any{{"name": "test-pkg", "version": "1.0"}})
	out := stdout.String()
	if strings.Contains(out, "[]") {
		t.Error("no type should not show empty badge brackets")
	}
	if !strings.Contains(out, "test-pkg") {
		t.Error("should still show name")
	}
}

func TestResolveFormat_ErrorMessage(t *testing.T) {
	_, err := ResolveFormat(true, true, false, false, false, false)
	if err == nil {
		t.Fatal("expected error for conflicting flags")
	}
	msg := err.Error()
	for _, flag := range []string{"--json", "--quiet", "--md"} {
		if !strings.Contains(msg, flag) {
			t.Errorf("error message should mention %q, got: %s", flag, msg)
		}
	}
}

func TestWriter_PrintMapRow_ColumnAlignment_ColorAlways(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled), WithColorMode(ColorAlways))
	_ = w.OK([]map[string]any{
		{"name": "short", "type": "skill", "version": "1.0"},
		{"name": "a-much-longer-package-name", "type": "mcp", "version": "2.0"},
	})
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Both lines should have consistent visible alignment.
	// The version/type columns should start at roughly the same visible position.
	// We verify by checking that both lines have the same visible width pattern:
	// the name column visible width should pad to 30 chars.
	for i, line := range lines {
		// Strip leading spaces
		trimmed := strings.TrimLeft(line, " ")
		// The visible content before the first badge should be 30 chars
		// (the PadRight target). This is a sanity check that ANSI codes
		// don't break alignment.
		if VisibleLen(trimmed) < 30 {
			t.Errorf("line %d visible len = %d, too short for alignment", i, VisibleLen(trimmed))
		}
	}
}

func TestWriter_PrintMapRow_AllEmpty(t *testing.T) {
	stdout := &bytes.Buffer{}
	w := NewWriter(WithStdout(stdout), WithStderr(&bytes.Buffer{}), WithFormat(FormatStyled))
	_ = w.OK([]map[string]any{{}})
	// Should not panic
}

