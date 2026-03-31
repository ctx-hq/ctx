package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/tui"
)

var (
	discoverKeyEnter = key.NewBinding(key.WithKeys("enter"))
	discoverKeyTab   = key.NewBinding(key.WithKeys("tab"))
)

// discoverItem wraps a PackageInfo for the list.
type discoverItem struct {
	pkg registry.PackageInfo
}

func (i discoverItem) FilterValue() string { return i.pkg.FullName }

// discoverDelegate renders discover results.
type discoverDelegate struct{}

func (d discoverDelegate) Height() int                              { return 2 }
func (d discoverDelegate) Spacing() int                             { return 0 }
func (d discoverDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d discoverDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(discoverItem)
	if !ok {
		return
	}

	title := tui.ListItemTitle.Render(it.pkg.FullName)
	badge := tui.TypeBadgeText(it.pkg.Type)

	cursor := "  "
	if index == m.Cursor() {
		cursor = "> "
	}

	fmt.Fprintf(w, "%s%s %s\n", cursor, title, badge)
	desc := it.pkg.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	fmt.Fprintf(w, "  %s", tui.ListItemDesc.Render(desc))
}

// discoverTab provides search and browse functionality.
type discoverTab struct {
	searchInput textinput.Model
	results     list.Model
	service     Service
	searching   bool
	focused     string // "search" or "results"
}

func newDiscoverTab(svc Service, width, height int) discoverTab {
	ti := textinput.New()
	ti.Placeholder = "Search packages..."
	ti.Prompt = "/ "
	ti.CharLimit = 100

	resultHeight := height - 3 // account for search input
	if resultHeight < 1 {
		resultHeight = 1
	}
	l := list.New(nil, discoverDelegate{}, width, resultHeight)
	l.Title = "Search Results"
	l.DisableQuitKeybindings()

	return discoverTab{
		searchInput: ti,
		results:     l,
		service:     svc,
		focused:     "search",
	}
}

func (t discoverTab) Init() tea.Cmd {
	return t.searchInput.Focus()
}

func (t discoverTab) Update(msg tea.Msg) (discoverTab, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultMsg:
		t.searching = false
		if msg.Err != nil {
			return t, nil
		}
		items := make([]list.Item, len(msg.Result.Packages))
		for i, p := range msg.Result.Packages {
			items[i] = discoverItem{pkg: p}
		}
		cmd := t.results.SetItems(items)
		t.focused = "results"
		return t, cmd

	case tea.KeyPressMsg:
		if t.focused == "search" {
			if key.Matches(msg, discoverKeyEnter) {
				query := t.searchInput.Value()
				if query == "" {
					return t, nil
				}
				t.searching = true
				svc := t.service
				// BubbleTea commands run in goroutines without a parent context;
				// use a timeout to bound in-flight requests on TUI exit.
				return t, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					result, err := svc.Search(ctx, query, "", 20, 0)
					return searchResultMsg{Result: result, Err: err}
				}
			}
			if key.Matches(msg, discoverKeyTab) {
				if len(t.results.Items()) > 0 {
					t.focused = "results"
					t.searchInput.Blur()
				}
				return t, nil
			}
			var cmd tea.Cmd
			t.searchInput, cmd = t.searchInput.Update(msg)
			return t, cmd
		}

		// focused == "results"
		if key.Matches(msg, discoverKeyTab) {
			t.focused = "search"
			return t, t.searchInput.Focus()
		}
	}

	if t.focused == "results" {
		var cmd tea.Cmd
		t.results, cmd = t.results.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t discoverTab) View() string {
	search := t.searchInput.View()
	var body string
	if t.searching {
		body = "  Searching..."
	} else if len(t.results.Items()) == 0 {
		body = "  Type a query and press Enter to search."
	} else {
		body = t.results.View()
	}
	return search + "\n" + body
}

// SetSize updates the component dimensions.
func (t *discoverTab) SetSize(w, h int) {
	t.searchInput.SetWidth(w - 4)
	resultHeight := h - 3
	if resultHeight < 1 {
		resultHeight = 1
	}
	t.results.SetWidth(w)
	t.results.SetHeight(resultHeight)
}
