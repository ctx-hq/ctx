package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// --- ParseColorMode tests ---

func TestParseColorMode_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  ColorMode
	}{
		{"auto", ColorAuto},
		{"always", ColorAlways},
		{"never", ColorNever},
	}
	for _, tt := range tests {
		got, err := ParseColorMode(tt.input)
		if err != nil {
			t.Errorf("ParseColorMode(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseColorMode(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseColorMode_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
		want  ColorMode
	}{
		{"Auto", ColorAuto},
		{"ALWAYS", ColorAlways},
		{"Never", ColorNever},
		{"NEVER", ColorNever},
	}
	for _, tt := range tests {
		got, err := ParseColorMode(tt.input)
		if err != nil {
			t.Errorf("ParseColorMode(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseColorMode(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseColorMode_Invalid(t *testing.T) {
	invalid := []string{"invalid", "", "true", "yes", "1", "on"}
	for _, input := range invalid {
		_, err := ParseColorMode(input)
		if err == nil {
			t.Errorf("ParseColorMode(%q) expected error, got nil", input)
		}
	}
}

func TestParseColorMode_ErrorMessage(t *testing.T) {
	_, err := ParseColorMode("bad")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, keyword := range []string{"auto", "always", "never"} {
		if !strings.Contains(msg, keyword) {
			t.Errorf("error message should mention %q, got: %s", keyword, msg)
		}
	}
}

// --- Styler tests ---

func TestStyler_NoColor_AllMethods(t *testing.T) {
	s := NewStyler(colorprofile.ASCII)
	methods := map[string]func(string) string{
		"Bold":    s.Bold,
		"Dim":     s.Dim,
		"Success": s.Success,
		"Error":   s.Error,
		"Warning": s.Warning,
		"Info":    s.Info,
		"Name":    s.Name,
	}
	for name, fn := range methods {
		got := fn("hello")
		if got != "hello" {
			t.Errorf("Styler.%s with ASCII profile should return original text, got %q", name, got)
		}
	}
}

func TestStyler_NoColor_EmptyString(t *testing.T) {
	s := NewStyler(colorprofile.ASCII)
	if got := s.Bold(""); got != "" {
		t.Errorf("Bold empty string should return empty, got %q", got)
	}
}

func TestStyler_NoColor_SpecialChars(t *testing.T) {
	s := NewStyler(colorprofile.ASCII)
	special := "hello\nworld\t\u4e2d\u6587"
	if got := s.Bold(special); got != special {
		t.Errorf("Bold special chars should return original, got %q", got)
	}
}

func TestStyler_NoColor_LongString(t *testing.T) {
	s := NewStyler(colorprofile.ASCII)
	long := strings.Repeat("x", 5000)
	if got := s.Bold(long); got != long {
		t.Errorf("Bold long string length = %d, want %d", len(got), len(long))
	}
}

func TestStyler_WithColor_ContainsANSI(t *testing.T) {
	s := NewStyler(colorprofile.TrueColor)
	methods := map[string]func(string) string{
		"Bold":    s.Bold,
		"Dim":     s.Dim,
		"Success": s.Success,
		"Error":   s.Error,
		"Warning": s.Warning,
		"Info":    s.Info,
		"Name":    s.Name,
	}
	for name, fn := range methods {
		got := fn("test")
		if !strings.Contains(got, "\033[") {
			t.Errorf("Styler.%s with TrueColor should contain ANSI escape, got %q", name, got)
		}
	}
}

func TestStyler_WithColor_ContainsOriginalText(t *testing.T) {
	s := NewStyler(colorprofile.TrueColor)
	got := s.Bold("original text")
	if !strings.Contains(got, "original text") {
		t.Errorf("styled output should contain original text, got %q", got)
	}
}

func TestStyler_TypeBadge_PerType(t *testing.T) {
	s := NewStyler(colorprofile.TrueColor)
	tests := []struct {
		typ  string
		want string // should contain this substring
	}{
		{"skill", "[skill]"},
		{"mcp", "[mcp]"},
		{"cli", "[cli]"},
		{"unknown", "[unknown]"},
	}
	for _, tt := range tests {
		got := s.TypeBadge(tt.typ)
		if !strings.Contains(got, tt.want) {
			t.Errorf("TypeBadge(%q) should contain %q, got %q", tt.typ, tt.want, got)
		}
	}
	// Known types should have ANSI when colored
	for _, typ := range []string{"skill", "mcp", "cli"} {
		got := s.TypeBadge(typ)
		if !strings.Contains(got, "\033[") {
			t.Errorf("TypeBadge(%q) with TrueColor should have ANSI, got %q", typ, got)
		}
	}
}

func TestStyler_TypeBadge_NoColor(t *testing.T) {
	s := NewStyler(colorprofile.ASCII)
	for _, typ := range []string{"skill", "mcp", "cli", "unknown"} {
		got := s.TypeBadge(typ)
		if strings.Contains(got, "\033[") {
			t.Errorf("TypeBadge(%q) with ASCII should have no ANSI, got %q", typ, got)
		}
	}
}

// --- Palette tests ---

// --- Environment variable priority chain tests ---
// These test the ColorMode → colorprofile.Profile mapping, which is what
// --color=never/always actually controls. The env var detection itself is
// handled by colorprofile.Detect and is tested in that library.

func TestFlag_ColorNever_ForcesASCII(t *testing.T) {
	// --color=never should always produce ASCII profile (no color)
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(&bytes.Buffer{}), WithColorMode(ColorNever))
	out := w.styler.Bold("test")
	if out != "test" {
		t.Errorf("ColorNever: Bold should return plain text, got %q", out)
	}
}

func TestFlag_ColorAlways_ForcesTrueColor(t *testing.T) {
	// --color=always should produce TrueColor profile (full color)
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(&bytes.Buffer{}), WithColorMode(ColorAlways))
	out := w.styler.Bold("test")
	if !strings.Contains(out, "\033[") {
		t.Errorf("ColorAlways: Bold should contain ANSI, got %q", out)
	}
}

func TestFlag_ColorNever_OverridesAll(t *testing.T) {
	// --color=never should disable color regardless of any other condition
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorNever))
	w.Success("test")
	w.Warn("test")
	w.Info("test")
	if strings.Contains(stderr.String(), "\033[") {
		t.Error("ColorNever should produce zero ANSI in all output methods")
	}
}

func TestFlag_ColorAlways_AllMethodsColored(t *testing.T) {
	// --color=always should produce color in all styled methods
	stderr := &bytes.Buffer{}
	w := NewWriter(WithStdout(&bytes.Buffer{}), WithStderr(stderr), WithColorMode(ColorAlways))
	w.Success("test")
	w.Warn("test")
	w.Info("test")
	if !strings.Contains(stderr.String(), "\033[") {
		t.Error("ColorAlways should produce ANSI in output methods")
	}
}

// --- VisibleLen / PadRight tests ---

func TestVisibleLen_PlainText(t *testing.T) {
	if got := VisibleLen("hello"); got != 5 {
		t.Errorf("VisibleLen plain = %d, want 5", got)
	}
}

func TestVisibleLen_WithANSI(t *testing.T) {
	styled := "\033[1mhello\033[0m"
	if got := VisibleLen(styled); got != 5 {
		t.Errorf("VisibleLen ANSI = %d, want 5", got)
	}
}

func TestVisibleLen_ComplexANSI(t *testing.T) {
	// Bold + foreground color + reset
	styled := "\033[1;38;2;95;175;255m[skill]\033[0m"
	if got := VisibleLen(styled); got != 7 {
		t.Errorf("VisibleLen complex ANSI = %d, want 7 (len of '[skill]')", got)
	}
}

func TestVisibleLen_Empty(t *testing.T) {
	if got := VisibleLen(""); got != 0 {
		t.Errorf("VisibleLen empty = %d, want 0", got)
	}
}

func TestPadRight_PlainText(t *testing.T) {
	got := PadRight("abc", 10)
	if len(got) != 10 {
		t.Errorf("PadRight plain len = %d, want 10", len(got))
	}
	if got != "abc       " {
		t.Errorf("PadRight plain = %q", got)
	}
}

func TestPadRight_WithANSI(t *testing.T) {
	styled := "\033[1mabc\033[0m"
	got := PadRight(styled, 10)
	// Visible width should be 10: "abc" + 7 spaces
	visible := VisibleLen(got)
	if visible != 10 {
		t.Errorf("PadRight ANSI visible = %d, want 10", visible)
	}
	// Must still contain the ANSI codes
	if !strings.Contains(got, "\033[1m") {
		t.Error("PadRight should preserve ANSI codes")
	}
}

func TestPadRight_AlreadyWide(t *testing.T) {
	got := PadRight("abcdefghij", 5)
	if got != "abcdefghij" {
		t.Errorf("PadRight should not truncate, got %q", got)
	}
}

func TestPadRight_ExactWidth(t *testing.T) {
	got := PadRight("abc", 3)
	if got != "abc" {
		t.Errorf("PadRight exact = %q, want %q", got, "abc")
	}
}

func TestPalette_AllFieldsNonNil(t *testing.T) {
	if Palette.Primary == nil {
		t.Error("Palette.Primary is nil")
	}
	if Palette.Secondary == nil {
		t.Error("Palette.Secondary is nil")
	}
	if Palette.Accent == nil {
		t.Error("Palette.Accent is nil")
	}
	if Palette.Success == nil {
		t.Error("Palette.Success is nil")
	}
	if Palette.Warning == nil {
		t.Error("Palette.Warning is nil")
	}
	if Palette.Error == nil {
		t.Error("Palette.Error is nil")
	}
	if Palette.Skill == nil {
		t.Error("Palette.Skill is nil")
	}
	if Palette.MCP == nil {
		t.Error("Palette.MCP is nil")
	}
	if Palette.CLI == nil {
		t.Error("Palette.CLI is nil")
	}
}
