package output

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/colorprofile"
)

// ColorMode represents the user's color preference from --color flag.
type ColorMode int

const (
	ColorAuto   ColorMode = iota // detect from TTY + env vars
	ColorAlways                  // force color on
	ColorNever                   // force color off
)

// ParseColorMode parses "auto", "always", "never" from the --color flag value.
func ParseColorMode(s string) (ColorMode, error) {
	switch strings.ToLower(s) {
	case "auto":
		return ColorAuto, nil
	case "always":
		return ColorAlways, nil
	case "never":
		return ColorNever, nil
	default:
		return ColorAuto, fmt.Errorf("invalid color mode %q: valid values are auto, always, never", s)
	}
}

// AdaptiveColor returns an AdaptiveColor that picks the right shade for
// dark vs light terminal backgrounds. All pairs should maintain WCAG 4.5:1
// contrast minimum against their respective backgrounds.
func AdaptiveColor(dark, light string) color.Color {
	return compat.AdaptiveColor{
		Dark:  lipgloss.Color(dark),
		Light: lipgloss.Color(light),
	}
}

// Palette holds the WCAG 4.5:1 compliant adaptive color palette shared between
// CLI output and TUI. These are the canonical color definitions — all styling
// in the codebase should reference these values.
var Palette = struct {
	Primary   color.Color // High contrast text
	Secondary color.Color // Muted/dimmed text
	Accent    color.Color // Active highlight
	Success   color.Color // Green
	Warning   color.Color // Yellow
	Error     color.Color // Red
	Skill     color.Color // Skill badge (blue)
	MCP       color.Color // MCP badge (purple)
	CLI       color.Color // CLI badge (orange)
}{
	Primary:   AdaptiveColor("#E0E0E0", "#1A1A1A"),
	Secondary: AdaptiveColor("#A0A0A0", "#555555"),
	Accent:    AdaptiveColor("#5FAFFF", "#0055AA"),
	Success:   AdaptiveColor("#5FD75F", "#007A00"),
	Warning:   AdaptiveColor("#D7AF5F", "#7A5500"),
	Error:     AdaptiveColor("#FF5F5F", "#B30000"),
	Skill:     AdaptiveColor("#87D7FF", "#004C80"),
	MCP:       AdaptiveColor("#D7AFFF", "#55007A"),
	CLI:       AdaptiveColor("#FFAF5F", "#7A4400"),
}

// Styler applies semantic colors to text. When color is disabled (profile ≤ ASCII),
// all methods return the original text unmodified.
type Styler struct {
	profile colorprofile.Profile

	// Pre-built styles (only used when color is active).
	boldStyle    lipgloss.Style
	dimStyle     lipgloss.Style
	successStyle lipgloss.Style
	errorStyle   lipgloss.Style
	warningStyle lipgloss.Style
	infoStyle    lipgloss.Style
	nameStyle    lipgloss.Style
	skillStyle   lipgloss.Style
	mcpStyle     lipgloss.Style
	cliStyle     lipgloss.Style
}

// NewStyler creates a Styler that respects the given color profile.
func NewStyler(profile colorprofile.Profile) *Styler {
	s := &Styler{profile: profile}
	if profile > colorprofile.ASCII {
		s.boldStyle = lipgloss.NewStyle().Bold(true)
		s.dimStyle = lipgloss.NewStyle().Faint(true)
		s.successStyle = lipgloss.NewStyle().Foreground(Palette.Success)
		s.errorStyle = lipgloss.NewStyle().Foreground(Palette.Error)
		s.warningStyle = lipgloss.NewStyle().Foreground(Palette.Warning)
		s.infoStyle = lipgloss.NewStyle().Foreground(Palette.Accent)
		s.nameStyle = lipgloss.NewStyle().Foreground(Palette.Primary).Bold(true)
		s.skillStyle = lipgloss.NewStyle().Foreground(Palette.Skill).Bold(true)
		s.mcpStyle = lipgloss.NewStyle().Foreground(Palette.MCP).Bold(true)
		s.cliStyle = lipgloss.NewStyle().Foreground(Palette.CLI).Bold(true)
	}
	return s
}

// Bold renders text in bold.
func (s *Styler) Bold(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.boldStyle.Render(text)
}

// Dim renders text in dimmed/faint style.
func (s *Styler) Dim(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.dimStyle.Render(text)
}

// Success renders text in success green.
func (s *Styler) Success(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.successStyle.Render(text)
}

// Error renders text in error red.
func (s *Styler) Error(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.errorStyle.Render(text)
}

// Warning renders text in warning yellow.
func (s *Styler) Warning(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.warningStyle.Render(text)
}

// Info renders text in info/accent color.
func (s *Styler) Info(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.infoStyle.Render(text)
}

// Name renders text as a package/item name (bold primary).
func (s *Styler) Name(text string) string {
	if s.profile <= colorprofile.ASCII {
		return text
	}
	return s.nameStyle.Render(text)
}

// TypeBadge renders a type badge with the appropriate color per type.
func (s *Styler) TypeBadge(typ string) string {
	badge := "[" + typ + "]"
	if s.profile <= colorprofile.ASCII {
		return badge
	}
	switch typ {
	case "skill":
		return s.skillStyle.Render(badge)
	case "mcp":
		return s.mcpStyle.Render(badge)
	case "cli":
		return s.cliStyle.Render(badge)
	default:
		return badge
	}
}

// ansiRe matches ANSI escape sequences for visible-width calculation.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// VisibleLen returns the visible character count of s, ignoring ANSI escapes.
func VisibleLen(s string) int {
	return len(ansiRe.ReplaceAllString(s, ""))
}

// PadRight pads s to the given visible width with spaces, correctly handling
// ANSI escape sequences that would break fmt's %-Ns formatting.
func PadRight(s string, width int) string {
	visible := VisibleLen(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}
