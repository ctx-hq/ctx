package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// View renders the full TUI.
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	if !m.ready {
		v := tea.NewView("  Loading...")
		v.AltScreen = true
		return v
	}

	var sections []string
	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderContent())
	sections = append(sections, m.statusBar.View())

	body := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.showHelp {
		body = m.renderHelpOverlay()
	}

	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m Model) renderHeader() string {
	switch m.mode {
	case modeInstalled, modeAgents:
		return m.renderTabHeader()
	case modeBrowse:
		title := m.browsePackage
		if m.browseDir != "" {
			// Show the last component of the path for brevity.
			title = filepath.Base(m.browseDir)
			if title == "." || title == "" {
				title = m.browsePackage
			}
		}
		return m.renderModeHeader(title, "esc:back  enter:open")
	case modeSearch:
		n := len(m.pkgList.Items())
		title := "SEARCH"
		if n > 0 {
			title = fmt.Sprintf("SEARCH (%d)", n)
		}
		return m.renderModeHeader(title, "esc:back")
	case modeDoctor:
		return m.renderModeHeader(fmt.Sprintf("DOCTOR (%d)", len(m.doctorList.Items())), "esc:back")
	}
	return ""
}

// renderTabHeader renders the PACKAGES / AGENTS tab bar.
func (m Model) renderTabHeader() string {
	pkgLabel := "PACKAGES"
	agentLabel := "AGENTS"

	var pkgTab, agentTab string
	if m.mode == modeInstalled {
		pkgTab = tui.ActiveTab.Render(pkgLabel)
		agentTab = tui.InactiveTab.Render(agentLabel)
	} else {
		pkgTab = tui.InactiveTab.Render(pkgLabel)
		agentTab = tui.ActiveTab.Render(agentLabel)
	}

	tabs := pkgTab + "  " + agentTab
	hints := tui.HelpStyle.Render("d:doctor  ?:help")

	gap := m.width - lipgloss.Width(tabs) - lipgloss.Width(hints)
	if gap < 1 {
		gap = 1
	}
	return tabs + strings.Repeat(" ", gap) + hints
}

// renderModeHeader renders a standard header with title and hints.
func (m Model) renderModeHeader(title, hints string) string {
	modeName := tui.ActiveTab.Render(" " + title + " ")
	hintsRendered := tui.HelpStyle.Render(" " + hints + " ")

	gap := m.width - lipgloss.Width(modeName) - lipgloss.Width(hintsRendered)
	if gap < 1 {
		gap = 1
	}
	return modeName + strings.Repeat(" ", gap) + hintsRendered
}

func (m Model) renderContent() string {
	contentHeight := m.height - headerHeight - footerHeight
	if contentHeight < 2 {
		contentHeight = 2
	}

	// Build left pane content.
	var leftParts []string
	if m.mode == modeSearch {
		// Render search input as plain text with cursor.
		prompt := " / "
		cursor := "█"
		if m.focus != focusSearch {
			cursor = ""
		}
		searchLine := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(prompt) +
			m.searchQuery + cursor
		leftParts = append(leftParts, searchLine)
	}

	switch m.mode {
	case modeInstalled, modeSearch:
		leftParts = append(leftParts, m.pkgList.View())
	case modeAgents:
		leftParts = append(leftParts, m.agentList.View())
	case modeDoctor:
		leftParts = append(leftParts, m.doctorList.View())
	case modeBrowse:
		leftParts = append(leftParts, m.fileList.View())
	}

	// Empty-state guidance.
	if m.mode == modeAgents && len(m.agentList.Items()) == 0 {
		leftParts = append(leftParts, "")
		leftParts = append(leftParts, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"  No agents detected\n\n  Install Claude Code, Cursor,\n  or Windsurf"))
	}
	if m.mode == modeInstalled && len(m.pkgList.Items()) == 0 {
		leftParts = append(leftParts, "")
		leftParts = append(leftParts, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"  No packages installed\n\n  ^F search registry\n  ? help"))
	}

	leftContent := lipgloss.JoinVertical(lipgloss.Left, leftParts...)

	// Narrow terminal: list only.
	if m.width < minDetailWidth {
		return lipgloss.NewStyle().Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
	}

	// Wide terminal: bordered left + bordered right.
	lw := m.getListWidth()
	innerLW := lw - 2
	if innerLW < 10 {
		innerLW = 10
	}

	// Focused pane gets brighter border.
	leftBorderColor := lipgloss.Color("237")
	rightBorderColor := lipgloss.Color("237")
	if m.focus == focusList || m.focus == focusSearch {
		leftBorderColor = lipgloss.Color("63")
	}
	if m.focus == focusDetail {
		rightBorderColor = lipgloss.Color("63")
	}

	leftBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorderColor).
		Width(innerLW).
		Height(contentHeight - 2).
		MaxHeight(contentHeight).
		Render(leftContent)

	dw := m.width - lw - 1
	if dw < 12 {
		dw = 12
	}
	innerDW := dw - 2
	if innerDW < 10 {
		innerDW = 10
	}

	// Show scroll hint when detail has more content.
	detailContent := m.detail.View()
	if m.focus != focusDetail && m.detail.TotalLineCount() > m.detail.Height() {
		detailContent += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  tab: scroll ↕")
	}

	rightBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor).
		Width(innerDW).
		Height(contentHeight - 2).
		MaxHeight(contentHeight).
		PaddingLeft(1).
		PaddingRight(1).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (m Model) renderHelpOverlay() string {
	helpText := `  ctx tui — Interactive Package Browser

  Navigation
    ↑/↓  j/k     Navigate list
    ←/→          Switch tabs (Packages ↔ Agents)
    tab          Focus detail pane (scroll)
    esc          Back

  Views
    ctrl+f       Search registry
    d            Diagnostics

  Actions
    enter        Browse package files
    y            Copy command to clipboard

  Other
    ?            Toggle this help
    q  ctrl+c    Quit`

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 3).
		Render(helpText)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
