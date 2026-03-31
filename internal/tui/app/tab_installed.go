package app

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/tui"
)

// installedItem wraps an InstalledPackage for the list.
type installedItem struct {
	pkg installer.InstalledPackage
}

func (i installedItem) FilterValue() string { return i.pkg.FullName }

// installedDelegate renders installed package items.
type installedDelegate struct{}

func (d installedDelegate) Height() int                              { return 2 }
func (d installedDelegate) Spacing() int                             { return 0 }
func (d installedDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d installedDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(installedItem)
	if !ok {
		return
	}

	title := tui.ListItemTitle.Render(it.pkg.FullName)
	badge := tui.TypeBadgeText(it.pkg.Type)
	version := tui.ListItemDesc.Render(it.pkg.Version)

	cursor := "  "
	if index == m.Cursor() {
		cursor = "> "
	}

	fmt.Fprintf(w, "%s%s %s\n", cursor, title, badge)
	fmt.Fprintf(w, "  %s", version)
}

// installedTab shows installed packages.
type installedTab struct {
	list    list.Model
	service Service
	loaded  bool
}

func newInstalledTab(svc Service, width, height int) installedTab {
	l := list.New(nil, installedDelegate{}, width, height)
	l.Title = "Installed Packages"
	l.DisableQuitKeybindings()
	return installedTab{
		list:    l,
		service: svc,
	}
}

func (t installedTab) Init() tea.Cmd {
	svc := t.service
	return func() tea.Msg {
		pkgs, err := svc.ScanInstalled()
		return installedLoadedMsg{Pkgs: pkgs, Err: err}
	}
}

func (t installedTab) Update(msg tea.Msg) (installedTab, tea.Cmd) {
	switch msg := msg.(type) {
	case installedLoadedMsg:
		t.loaded = true
		if msg.Err != nil {
			return t, nil
		}
		items := make([]list.Item, len(msg.Pkgs))
		for i, p := range msg.Pkgs {
			items[i] = installedItem{pkg: p}
		}
		cmd := t.list.SetItems(items)
		return t, cmd
	}

	var cmd tea.Cmd
	t.list, cmd = t.list.Update(msg)
	return t, cmd
}

func (t installedTab) View() string {
	if !t.loaded {
		return "  Loading installed packages..."
	}
	if len(t.list.Items()) == 0 {
		return "  No packages installed.\n\n  Run 'ctx install <package>' to get started."
	}
	return t.list.View()
}

// SetSize updates the list dimensions.
func (t *installedTab) SetSize(w, h int) {
	t.list.SetWidth(w)
	t.list.SetHeight(h)
}
