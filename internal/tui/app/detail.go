package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"

	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/tui"
)

// ── Styles for detail pane ──

var (
	detailTitle = lipgloss.NewStyle().Bold(true)
	detailLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(12)
	detailValue = lipgloss.NewStyle()
	detailSection = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			Bold(true).
			MarginTop(1)
	detailDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	detailOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	detailWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	detailFail = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "ok", "pass":
		return detailOK
	case "warn", "broken", "missing":
		return detailWarn
	case "fail", "failed":
		return detailFail
	default:
		return detailValue
	}
}

func kvLine(label, value string) string {
	return detailLabel.Render(label) + detailValue.Render(value)
}

func sectionHeader(title string) string {
	return detailSection.Render("── " + title + " ──")
}

// Cached glamour renderer — creating one per render is the #1 bottleneck.
var (
	cachedRenderer      *glamour.TermRenderer
	cachedRendererWidth int
)

func getRenderer(width int) *glamour.TermRenderer {
	if cachedRenderer != nil && cachedRendererWidth == width {
		return cachedRenderer
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	cachedRenderer = r
	cachedRendererWidth = width
	return r
}

// renderMarkdown renders markdown content using glamour, falling back to raw text on error.
func renderMarkdown(content string, width int) string {
	r := getRenderer(width)
	if r == nil {
		return content
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

// langFromExt returns a markdown code fence language from a file extension.
func langFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "go"
	case ".sh", ".bash":
		return "bash"
	case ".py":
		return "python"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".lua":
		return "lua"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".md":
		return "markdown"
	default:
		return ""
	}
}

// maxRenderLines limits glamour rendering to avoid slowness on huge files.
const maxRenderLines = 500

// renderFileContent renders file content for the detail pane.
// Markdown files are rendered with glamour directly.
// Code files are wrapped in a code fence and rendered.
// Other files are shown as plain text.
func renderFileContent(name, content string, width int) string {
	// Truncate very large content before rendering.
	truncated := false
	if lines := strings.Split(content, "\n"); len(lines) > maxRenderLines {
		content = strings.Join(lines[:maxRenderLines], "\n")
		truncated = true
	}

	ext := strings.ToLower(filepath.Ext(name))

	var result string
	if ext == ".md" {
		// Only .md files get glamour rendering (the only slow path).
		result = renderMarkdown(content, width)
	} else {
		// All other files: raw text, instant.
		result = content
	}

	if truncated {
		result += "\n\n" + detailDim.Render(fmt.Sprintf("... truncated at %d lines", maxRenderLines))
	}
	return result
}

// ── Package detail ──

func renderPkgDetail(item pkgItem, width int, state *installstate.PackageState, skillContent string) string {
	var lines []string

	// Title
	lines = append(lines, detailTitle.Render(item.fullName))
	lines = append(lines, "")

	// Metadata
	lines = append(lines, kvLine("Type", tui.TypeBadgeText(item.pkgType)))
	lines = append(lines, kvLine("Version", item.version))
	if item.installed {
		lines = append(lines, kvLine("Status", detailOK.Render("✓ installed")))
	}

	// Description
	if item.description != "" {
		lines = append(lines, "")
		desc := lipgloss.NewStyle().Width(width).Render(item.description)
		lines = append(lines, desc)
	}

	// Agent linkage from install state
	if state != nil && len(state.Skills) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("Linked Agents"))
		for _, s := range state.Skills {
			icon := statusStyle(s.Status).Render(tui.IconPass)
			if s.Status != "ok" {
				icon = statusStyle(s.Status).Render(tui.IconWarn)
			}
			lines = append(lines, fmt.Sprintf("  %s %s", icon, s.Agent))
			lines = append(lines, detailDim.Render("    "+s.SymlinkPath))
		}
	}

	// MCP config from install state
	if state != nil && len(state.MCP) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("MCP Config"))
		for _, m := range state.MCP {
			icon := statusStyle(m.Status).Render(tui.IconPass)
			if m.Status != "ok" {
				icon = statusStyle(m.Status).Render(tui.IconWarn)
			}
			lines = append(lines, fmt.Sprintf("  %s %s → %s", icon, m.Agent, m.ConfigKey))
		}
	}

	// CLI info from install state
	if state != nil && state.CLI != nil {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("CLI"))
		cli := state.CLI
		lines = append(lines, kvLine("Binary", cli.Binary))
		if cli.BinaryPath != "" {
			lines = append(lines, kvLine("Path", cli.BinaryPath))
		}
		lines = append(lines, kvLine("Adapter", cli.Adapter))
		icon := statusStyle(cli.Status).Render(tui.IconPass)
		if cli.Status != "ok" {
			icon = statusStyle(cli.Status).Render(tui.IconFail)
		}
		lines = append(lines, kvLine("Status", icon+" "+cli.Status))
	}

	// SKILL.md content rendered with glamour
	if skillContent != "" {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("Documentation"))
		rendered := renderMarkdown(skillContent, width)
		lines = append(lines, rendered)
	}

	// Actions
	lines = append(lines, "")
	lines = append(lines, sectionHeader("Actions"))
	if item.installed {
		lines = append(lines, detailDim.Render("  ↵   browse package files"))
		lines = append(lines, detailDim.Render("  y   copy remove command"))
	} else {
		lines = append(lines, detailDim.Render("  y   copy install command"))
		lines = append(lines, detailDim.Render("  ↵   show command in status bar"))
	}

	return strings.Join(lines, "\n")
}

// ── Agent detail ──

func renderAgentDetail(item agentItem, _ int, skills []AgentSkillEntry, mcpServers []AgentMCPEntry) string {
	var lines []string

	lines = append(lines, detailTitle.Render(item.name))
	lines = append(lines, "")
	lines = append(lines, kvLine("Skills", fmt.Sprintf("%d installed", item.skillCount)))
	lines = append(lines, kvLine("Directory", item.skillsDir))

	// Skills section.
	if len(skills) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("Skills"))
		for _, s := range skills {
			if s.IsSymlink {
				lines = append(lines, fmt.Sprintf("  %s → %s", s.Name, s.LinkTarget))
			} else {
				lines = append(lines, fmt.Sprintf("  %s", s.Name))
			}
		}
	}

	// MCP servers section.
	if len(mcpServers) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("MCP Servers"))
		for _, mcp := range mcpServers {
			lines = append(lines, fmt.Sprintf("  %s", mcp.Name))
			lines = append(lines, detailDim.Render(fmt.Sprintf("    %s", mcp.Command)))
		}
	}

	// Actions.
	lines = append(lines, "")
	lines = append(lines, sectionHeader("Actions"))
	lines = append(lines, detailDim.Render("  ↵   browse skills directory"))

	return strings.Join(lines, "\n")
}

// ── Doctor detail ──

func renderDoctorDetail(item doctorItem, width int) string {
	var lines []string

	icon := statusStyle(item.status).Render(tui.IconPass)
	switch item.status {
	case "warn":
		icon = statusStyle(item.status).Render(tui.IconWarn)
	case "fail":
		icon = statusStyle(item.status).Render(tui.IconFail)
	}

	lines = append(lines, detailTitle.Render(icon+" "+item.name))
	lines = append(lines, "")
	lines = append(lines, kvLine("Status", statusStyle(item.status).Render(item.status)))

	if item.detail != "" {
		lines = append(lines, "")
		detail := lipgloss.NewStyle().Width(width).Render(item.detail)
		lines = append(lines, detail)
	}

	if item.hint != "" {
		lines = append(lines, "")
		lines = append(lines, sectionHeader("Hint"))
		lines = append(lines, detailDim.Render("  "+item.hint))
	}

	return strings.Join(lines, "\n")
}

// ── Empty states ──

func renderEmptyDetail(mode viewMode, _ int) string {
	switch mode {
	case modeInstalled:
		return strings.Join([]string{
			detailDim.Render("No package selected"),
			"",
			detailDim.Render("Install packages with:"),
			detailValue.Render("  ctx install @scope/name"),
			"",
			detailDim.Render("Or press / to search the registry"),
		}, "\n")
	case modeSearch:
		return strings.Join([]string{
			detailDim.Render("Type a search query and press Enter"),
			"",
			detailDim.Render("Examples:"),
			detailValue.Render("  code review"),
			detailValue.Render("  github mcp"),
			detailValue.Render("  file search cli"),
		}, "\n")
	case modeAgents:
		return detailDim.Render("No agents detected.\nInstall Claude Code, Cursor, or Windsurf.")
	case modeDoctor:
		return detailDim.Render("Running diagnostics...")
	case modeBrowse:
		return detailDim.Render("Select a file to view its content")
	default:
		return ""
	}
}
