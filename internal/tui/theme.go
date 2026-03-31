// Package tui provides terminal user interface components for the ctx CLI.
package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// MinWidth is the minimum terminal width required for the TUI.
const MinWidth = 40

// Status icon constants for use in status displays.
const (
	IconPass = "✓"
	IconWarn = "!"
	IconFail = "✗"
)

// adaptiveColor returns an AdaptiveColor that picks the right shade for
// dark vs light terminal backgrounds. All pairs maintain WCAG 4.5:1
// contrast minimum against their respective backgrounds.
func adaptiveColor(dark, light string) color.Color {
	return compat.AdaptiveColor{
		Dark:  lipgloss.Color(dark),
		Light: lipgloss.Color(light),
	}
}

// --- Color palette (WCAG 4.5:1 contrast on respective backgrounds) ---

var (
	// Primary text: high contrast on both backgrounds.
	colorPrimary = adaptiveColor("#E0E0E0", "#1A1A1A")
	// Secondary/muted text.
	colorSecondary = adaptiveColor("#A0A0A0", "#555555")
	// Accent / active highlight.
	colorAccent = adaptiveColor("#5FAFFF", "#0055AA")
	// Success green.
	colorSuccess = adaptiveColor("#5FD75F", "#007A00")
	// Warning yellow.
	colorWarning = adaptiveColor("#D7AF5F", "#7A5500")
	// Error red.
	colorError = adaptiveColor("#FF5F5F", "#B30000")
	// Badge colors per type.
	colorSkill = adaptiveColor("#87D7FF", "#004C80")
	colorMCP   = adaptiveColor("#D7AFFF", "#55007A")
	colorCLI   = adaptiveColor("#FFAF5F", "#7A4400")
	// Tab bar / status bar backgrounds.
	colorBarBg    = adaptiveColor("#303030", "#D0D0D0")
	colorTabBg    = adaptiveColor("#404040", "#C0C0C0")
	colorActiveBg = adaptiveColor("#005FAF", "#B0D0FF")
)

// --- Styles ---

// ActiveTab is the style for the currently selected tab.
var ActiveTab = lipgloss.NewStyle().
	Foreground(adaptiveColor("#FFFFFF", "#000000")).
	Background(colorActiveBg).
	Bold(true).
	Padding(0, 2)

// InactiveTab is the style for unselected tabs.
var InactiveTab = lipgloss.NewStyle().
	Foreground(colorSecondary).
	Background(colorTabBg).
	Padding(0, 2)

// TabBar is the style for the full tab bar row.
var TabBar = lipgloss.NewStyle().
	Background(colorBarBg)

// StatusBar is the style for the status bar background.
var StatusBar = lipgloss.NewStyle().
	Background(colorBarBg).
	Foreground(colorPrimary)

// StatusBarLeft is the style for the left section of the status bar.
var StatusBarLeft = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)

// StatusBarRight is the style for the right section of the status bar.
var StatusBarRight = lipgloss.NewStyle().
	Foreground(colorSecondary)

// ListItemTitle is the style for list item titles.
var ListItemTitle = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// ListItemDesc is the style for list item descriptions.
var ListItemDesc = lipgloss.NewStyle().
	Foreground(colorSecondary)

// TypeBadgeSkill is the style for skill type badges.
var TypeBadgeSkill = lipgloss.NewStyle().
	Foreground(colorSkill).
	Bold(true)

// TypeBadgeMCP is the style for MCP server type badges.
var TypeBadgeMCP = lipgloss.NewStyle().
	Foreground(colorMCP).
	Bold(true)

// TypeBadgeCLI is the style for CLI tool type badges.
var TypeBadgeCLI = lipgloss.NewStyle().
	Foreground(colorCLI).
	Bold(true)

// DetailBorder is the style for the detail panel border.
var DetailBorder = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorAccent)

// HelpStyle is the style for help text at the bottom.
var HelpStyle = lipgloss.NewStyle().
	Foreground(colorSecondary)

// SuccessStyle is the style for success messages.
var SuccessStyle = lipgloss.NewStyle().
	Foreground(colorSuccess)

// WarningStyle is the style for warning messages.
var WarningStyle = lipgloss.NewStyle().
	Foreground(colorWarning)

// ErrorStyle is the style for error messages.
var ErrorStyle = lipgloss.NewStyle().
	Foreground(colorError)

// TypeBadgeText returns a text-based badge for the given package type.
// Returns "[skill]", "[mcp]", or "[cli]". Unknown types return the type
// wrapped in brackets.
func TypeBadgeText(t string) string {
	switch t {
	case "skill":
		return TypeBadgeSkill.Render("[skill]")
	case "mcp":
		return TypeBadgeMCP.Render("[mcp]")
	case "cli":
		return TypeBadgeCLI.Render("[cli]")
	default:
		return "[" + t + "]"
	}
}
