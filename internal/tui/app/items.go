package app

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/tui"
)

// fileDelegate renders file items in a compact single-line format.
type fileDelegate struct{}

func (d fileDelegate) Height() int                             { return 1 }
func (d fileDelegate) Spacing() int                            { return 0 }
func (d fileDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d fileDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(fileItem)
	if !ok {
		return
	}

	selected := index == m.Cursor()

	var line string
	if fi.isDir {
		name := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render(fi.name + "/")
		line = " 📁 " + name
	} else {
		size := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(formatFileSize(fi.size))
		line = " 📄 " + fi.name + "  " + size
	}

	if selected {
		cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render(">")
		line = cursor + line[1:] // replace leading space with cursor
	}

	fmt.Fprint(w, line)
}

// pkgItem implements list.DefaultItem for packages (both installed and search results).
type pkgItem struct {
	fullName    string
	version     string
	pkgType     string
	description string
	installed   bool
	installPath string
}

// fileItem implements list.DefaultItem for file browsing.
type fileItem struct {
	name  string
	size  int64
	path  string
	isDir bool
}

func (i fileItem) Title() string {
	if i.isDir {
		return "📁 " + i.name + "/"
	}
	return "📄 " + i.name
}
func (i fileItem) Description() string {
	if i.isDir {
		return ""
	}
	return formatFileSize(i.size)
}
func (i fileItem) FilterValue() string { return i.name }

func formatFileSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}

func (i pkgItem) Title() string { return i.fullName }
func (i pkgItem) Description() string {
	badge := tui.TypeBadgeText(i.pkgType)
	if i.version != "" {
		return badge + " " + i.version + "  " + i.description
	}
	return badge + "  " + i.description
}
func (i pkgItem) FilterValue() string { return i.fullName + " " + i.description }

// doctorItem implements list.DefaultItem for diagnostic checks.
type doctorItem struct {
	name   string
	status string
	detail string
	hint   string
}

func (i doctorItem) Title() string {
	icon := tui.IconPass
	switch i.status {
	case "warn":
		icon = tui.IconWarn
	case "fail":
		icon = tui.IconFail
	}
	return icon + " " + i.name
}
func (i doctorItem) Description() string { return i.detail }
func (i doctorItem) FilterValue() string { return i.name + " " + i.detail }

// agentItem implements list.DefaultItem for detected agents.
type agentItem struct {
	name       string
	skillsDir  string
	skillCount int
}

func (i agentItem) Title() string       { return i.name }
func (i agentItem) Description() string { return fmt.Sprintf("%d skills · %s", i.skillCount, i.skillsDir) }
func (i agentItem) FilterValue() string { return i.name }
