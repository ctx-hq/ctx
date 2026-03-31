// Package app provides the root TUI model for ctx — a dual-pane package browser.
package app

import (
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/tui/component"
)

// viewMode represents the current browsing mode.
type viewMode int

const (
	modeInstalled viewMode = iota
	modeAgents
	modeSearch
	modeDoctor
	modeBrowse
)

// focusArea represents which component has keyboard focus.
type focusArea int

const (
	focusList   focusArea = iota
	focusDetail           // right pane — viewport scrolling
	focusSearch           // search input
)

// Key bindings.
var (
	keyQuit      = key.NewBinding(key.WithKeys("q"))
	keyForceQuit = key.NewBinding(key.WithKeys("ctrl+c"))
	keyHelp      = key.NewBinding(key.WithKeys("?"))
	keySearch    = key.NewBinding(key.WithKeys("ctrl+f"))
	keyDoctor    = key.NewBinding(key.WithKeys("d"))
	keyEsc       = key.NewBinding(key.WithKeys("esc"))
	keyEnter     = key.NewBinding(key.WithKeys("enter"))
	keyYank      = key.NewBinding(key.WithKeys("y"))
	keyTab       = key.NewBinding(key.WithKeys("tab"))
	keyLeft      = key.NewBinding(key.WithKeys("left"))
	keyRight     = key.NewBinding(key.WithKeys("right"))
)

// Model is the root TUI model — a dual-pane package browser.
type Model struct {
	mode  viewMode
	focus focusArea

	// Left pane: one list active per mode.
	pkgList    list.Model
	agentList  list.Model
	doctorList list.Model
	fileList   list.Model

	// Right pane.
	detail viewport.Model

	// Search (plain string, no textinput.Model — avoids terminal CPR issues).
	searchQuery string

	// Help overlay.
	showHelp bool

	// Agent tab filter (-1 = All, 0+ = index into agents).
	agentFilter int

	// Browse mode state.
	browsePackage  string
	browseDir      string     // current directory being browsed
	browseDirStack []string   // parent directories for Esc navigation
	browseOrigin   viewMode   // mode to return to when exiting browse

	// Data cache.
	agents       []agentItem
	installed    []pkgItem
	stateCache   map[string]*installstate.PackageState // fullName → state
	skillCache   map[string]string                     // fullName → SKILL.md content
	renderCache  map[string]string                     // "dir:filename" → rendered content
	doctorLoaded bool

	// Infrastructure.
	statusBar   component.StatusBarModel
	service     Service
	startTime   time.Time // startup grace period: ignore keys for 500ms
	lastKeyTime time.Time // debounce: discard keys arriving < 5ms apart (terminal noise)
	noDebounce  bool      // disable debounce for testing
	width     int
	height    int
	ready     bool
	quitting  bool
}

func newList(delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.SetShowPagination(true)
	// Rebind page nav without left/right — we use arrows for tab switching.
	l.KeyMap.PrevPage.SetKeys("pgup", "b", "u")
	l.KeyMap.NextPage.SetKeys("pgdown", "f")
	return l
}

// New creates a new TUI model.
func New(svc Service) Model {
	delegate := list.NewDefaultDelegate()

	sb := component.NewStatusBar()
	sb.SetRight("←→:switch  tab:detail  d:doctor  ?:help  q:quit")

	return Model{
		mode:        modeInstalled,
		focus:       focusList,
		agentFilter: -1,
		pkgList:    newList(delegate),
		agentList:  newList(delegate),
		doctorList: newList(delegate),
		fileList:   newList(fileDelegate{}),
		detail:      viewport.New(),
		statusBar:   sb,
		service:     svc,
		startTime:  time.Now(),
		stateCache:  make(map[string]*installstate.PackageState),
		skillCache:  make(map[string]string),
		renderCache: make(map[string]string),
	}
}

// Init returns initial commands to load data.
// Doctor checks are loaded lazily when user presses 'd' (avoids slow HTTP/exec at startup).
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadInstalled(),
		m.loadAgents(),
	)
}

// ── Cached accessors ──

// getPackageState returns the cached install state, loading it on first access.
func (m *Model) getPackageState(fullName string) *installstate.PackageState {
	if s, ok := m.stateCache[fullName]; ok {
		return s
	}
	if m.service == nil {
		return nil
	}
	s := m.service.GetPackageState(fullName)
	m.stateCache[fullName] = s
	return s
}

// getSkillContent returns the cached SKILL.md content, loading it on first access.
func (m *Model) getSkillContent(fullName string) string {
	if c, ok := m.skillCache[fullName]; ok {
		return c
	}
	if m.service == nil {
		return ""
	}
	c := m.service.GetSkillContent(fullName)
	m.skillCache[fullName] = c
	return c
}
