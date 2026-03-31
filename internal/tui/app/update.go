package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/installstate"
)

// ── Commands (async data loading) ──

func (m Model) loadInstalled() tea.Cmd {
	return func() tea.Msg {
		pkgs, err := m.service.ScanInstalled()
		return installedLoadedMsg{Pkgs: pkgs, Err: err}
	}
}

func (m Model) loadAgents() tea.Cmd {
	return func() tea.Msg {
		return agentsDetectedMsg{Agents: m.service.DetectAgents()}
	}
}

func (m Model) loadDoctor() tea.Cmd {
	return func() tea.Msg {
		return doctorResultMsg{Result: m.service.RunDoctorChecks()}
	}
}

func (m Model) searchPackages(query string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.service.Search(context.Background(), query, "", 50, 0)
		return searchResultMsg{Result: result, Err: err}
	}
}

func (m Model) loadPackageFiles(fullName string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.service.ListPackageFiles(fullName)
		return filesLoadedMsg{Files: files, Err: err}
	}
}

func (m Model) loadFileContent(fullName, fileName string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.service.ReadPackageFile(fullName, fileName)
		return fileContentMsg{Name: fileName, Content: content, Err: err}
	}
}

func (m Model) loadDirFiles(dir string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.service.ListDirFiles(dir)
		return filesLoadedMsg{Files: files, Err: err}
	}
}

func (m Model) loadDirFileContent(dir, fileName string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.service.ReadDirFile(dir, fileName)
		return fileContentMsg{Name: fileName, Content: content, Err: err}
	}
}

// ── Update ──

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		m.updateDetailContent()
		return m, nil

	case installedLoadedMsg:
		return m.handleInstalledLoaded(msg)
	case searchResultMsg:
		return m.handleSearchResult(msg)
	case agentsDetectedMsg:
		return m.handleAgentsDetected(msg)
	case doctorResultMsg:
		return m.handleDoctorResult(msg)
	case filesLoadedMsg:
		return m.handleFilesLoaded(msg)
	case fileContentMsg:
		return m.handleFileContent(msg)

	case renderedContentMsg:
		return m.handleRenderedContent(msg)

	case statusMsg:
		m.statusBar.SetStatus(msg.Text)
		return m, nil

	case tea.KeyPressMsg:
		now := time.Now()

		// Guard 1: startup grace period (500ms).
		// BubbleTea sends terminal queries at startup; responses leak as
		// spurious key events (e.g. "2c2c/3434[2;1R").
		if now.Sub(m.startTime) < 500*time.Millisecond {
			return m, nil
		}

		// Guard 2: debounce rapid-fire key events.
		// Terminal query responses (CPR, DA) arrive as a burst of
		// characters within microseconds. Real key presses are > 15ms
		// apart.
		if !m.noDebounce {
			prevKeyTime := m.lastKeyTime
			m.lastKeyTime = now
			if !prevKeyTime.IsZero() {
				elapsed := now.Sub(prevKeyTime)
				if elapsed > 0 && elapsed < 5*time.Millisecond {
					return m, nil
				}
			}
		}

		return m.handleKeyPress(msg)
	}

	// Delegate non-key messages to active list (spinners, etc.)
	return m.delegateToActiveList(msg)
}

// ── Message handlers ──

func (m Model) handleInstalledLoaded(msg installedLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusBar.SetStatus("Error: " + msg.Err.Error())
		return m, nil
	}
	items := make([]list.Item, len(msg.Pkgs))
	m.installed = make([]pkgItem, len(msg.Pkgs))
	for i, p := range msg.Pkgs {
		pi := pkgItem{
			fullName:    p.FullName,
			version:     p.Version,
			pkgType:     p.Type,
			description: p.Description,
			installed:   true,
			installPath: p.InstallPath,
		}
		items[i] = pi
		m.installed[i] = pi
	}
	// Pre-cache package states to avoid lag on mode switch.
	for _, pi := range m.installed {
		m.getPackageState(pi.fullName)
	}
	cmd := m.pkgList.SetItems(items)
	m.statusBar.SetLeft(fmt.Sprintf("%d packages · %d agents", len(items), len(m.agents)))
	m.updateDetailContent()
	return m, cmd
}

func (m Model) handleSearchResult(msg searchResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusBar.SetStatus("Search error: " + msg.Err.Error())
		return m, nil
	}
	if msg.Result == nil || m.mode != modeSearch {
		return m, nil
	}
	items := make([]list.Item, len(msg.Result.Packages))
	for i, p := range msg.Result.Packages {
		items[i] = pkgItem{
			fullName:    p.FullName,
			version:     p.Version,
			pkgType:     p.Type,
			description: p.Description,
		}
	}
	cmd := m.pkgList.SetItems(items)
	m.statusBar.SetStatus(fmt.Sprintf("%d results", len(items)))
	m.updateDetailContent()
	return m, cmd
}

func (m Model) handleAgentsDetected(msg agentsDetectedMsg) (tea.Model, tea.Cmd) {
	m.agents = make([]agentItem, len(msg.Agents))
	items := make([]list.Item, len(msg.Agents))
	for i, a := range msg.Agents {
		ai := agentItem{name: a.Name, skillsDir: a.SkillsDir, skillCount: a.SkillCount}
		m.agents[i] = ai
		items[i] = ai
	}
	cmd := m.agentList.SetItems(items)
	m.statusBar.SetLeft(fmt.Sprintf("%d packages · %d agents", len(m.installed), len(m.agents)))
	return m, cmd
}

func (m Model) handleDoctorResult(msg doctorResultMsg) (tea.Model, tea.Cmd) {
	if msg.Result == nil {
		return m, nil
	}
	items := make([]list.Item, len(msg.Result.Checks))
	for i, c := range msg.Result.Checks {
		items[i] = doctorItem{name: c.Name, status: c.Status, detail: c.Detail, hint: c.Hint}
	}
	return m, m.doctorList.SetItems(items)
}

func (m Model) handleFilesLoaded(msg filesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusBar.SetStatus("Error loading files: " + msg.Err.Error())
		return m, nil
	}
	// Sort: directories first, then files, both alphabetically.
	sort.Slice(msg.Files, func(i, j int) bool {
		if msg.Files[i].IsDir != msg.Files[j].IsDir {
			return msg.Files[i].IsDir // dirs first
		}
		return msg.Files[i].Name < msg.Files[j].Name
	})
	// Prepend "../" entry for navigating up (unless at root browse level).
	var items []list.Item
	if len(m.browseDirStack) > 0 {
		items = append(items, fileItem{name: "..", isDir: true})
	}
	for _, f := range msg.Files {
		items = append(items, fileItem{name: f.Name, size: f.Size, isDir: f.IsDir})
	}
	setCmd := m.fileList.SetItems(items)
	m.updateDetailContent()
	// Auto-preview the first non-directory file.
	var previewCmd tea.Cmd
	for _, f := range msg.Files {
		if !f.IsDir {
			dir := m.browseDir
			if dir == "" {
				dir = m.resolvePackageDir()
			}
			if dir != "" {
				previewCmd = m.loadDirFileContent(dir, f.Name)
			}
			break
		}
	}
	if previewCmd != nil {
		return m, tea.Batch(setCmd, previewCmd)
	}
	return m, setCmd
}

func (m Model) renderCacheKey(name string) string {
	dir := m.browseDir
	if dir == "" {
		dir = m.resolvePackageDir()
	}
	return dir + ":" + name
}

func (m Model) handleFileContent(msg fileContentMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusBar.SetStatus("Error reading file: " + msg.Err.Error())
		return m, nil
	}

	cacheKey := m.renderCacheKey(msg.Name)

	// Check render cache first — instant.
	if cached, ok := m.renderCache[cacheKey]; ok {
		m.detail.SetContent(cached)
		m.detail.GotoTop()
		return m, m.preRenderAdjacent()
	}

	name := msg.Name
	content := msg.Content
	dw := m.detailWidth()

	// Non-.md files don't need glamour — render inline, instant.
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".md" {
		rendered := renderFileContent(name, content, dw)
		m.renderCache[cacheKey] = rendered
		m.detail.SetContent(rendered)
		m.detail.GotoTop()
		return m, m.preRenderAdjacent()
	}

	// .md files: show raw text immediately, glamour render in background.
	raw := content
	if lines := strings.SplitN(raw, "\n", 51); len(lines) > 50 {
		raw = strings.Join(lines[:50], "\n") + "\n..."
	}
	m.detail.SetContent(raw)
	m.detail.GotoTop()

	return m, func() tea.Msg {
		rendered := renderFileContent(name, content, dw)
		return renderedContentMsg{Key: cacheKey, Content: rendered}
	}
}

func (m Model) handleRenderedContent(msg renderedContentMsg) (tea.Model, tea.Cmd) {
	m.renderCache[msg.Key] = msg.Content
	// Only update the detail pane if this is the currently selected file.
	if item, ok := m.fileList.SelectedItem().(fileItem); ok {
		currentKey := m.renderCacheKey(item.name)
		if currentKey == msg.Key {
			m.detail.SetContent(msg.Content)
			m.detail.GotoTop()
		}
	}
	return m, m.preRenderAdjacent()
}

// preRenderAdjacent triggers background loading of prev/next file content.
func (m Model) preRenderAdjacent() tea.Cmd {
	if m.mode != modeBrowse {
		return nil
	}
	items := m.fileList.Items()
	idx := m.fileList.Cursor()
	dir := m.browseDir
	if dir == "" {
		dir = m.resolvePackageDir()
	}
	if dir == "" {
		return nil
	}

	var cmds []tea.Cmd
	for _, offset := range []int{-1, 1} {
		adjIdx := idx + offset
		if adjIdx < 0 || adjIdx >= len(items) {
			continue
		}
		fi, ok := items[adjIdx].(fileItem)
		if !ok || fi.isDir || fi.name == ".." {
			continue
		}
		cacheKey := dir + ":" + fi.name
		if _, cached := m.renderCache[cacheKey]; cached {
			continue
		}
		// Pre-load the file content (rendering happens when fileContentMsg arrives).
		name := fi.name
		cmds = append(cmds, m.loadDirFileContent(dir, name))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) delegateToActiveList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.mode {
	case modeInstalled, modeSearch:
		m.pkgList, cmd = m.pkgList.Update(msg)
	case modeAgents:
		m.agentList, cmd = m.agentList.Update(msg)
	case modeDoctor:
		m.doctorList, cmd = m.doctorList.Update(msg)
	case modeBrowse:
		m.fileList, cmd = m.fileList.Update(msg)
	}
	return m, cmd
}

// ── Key handling ──

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		if key.Matches(msg, keyEsc) || key.Matches(msg, keyHelp) {
			m.showHelp = false
		}
		return m, nil
	}

	if key.Matches(msg, keyForceQuit) {
		m.quitting = true
		return m, tea.Quit
	}

	if m.focus == focusSearch {
		return m.handleSearchInput(msg)
	}

	// Detail pane focused — scroll viewport, Tab/Esc returns to list.
	if m.focus == focusDetail {
		switch {
		case key.Matches(msg, keyTab), key.Matches(msg, keyEsc):
			m.focus = focusList
			return m, nil
		case key.Matches(msg, keyQuit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, keyHelp):
			m.showHelp = true
			return m, nil
		default:
			// Route j/k/up/down/pgup/pgdn to viewport for scrolling.
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}
	}

	// List focused.
	switch {
	case key.Matches(msg, keyQuit):
		if m.mode == modeBrowse {
			return m.exitBrowseMode()
		}
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, keyHelp):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, keySearch):
		if m.mode == modeBrowse {
			return m, nil
		}
		return m.enterSearchMode()

	case key.Matches(msg, keyLeft):
		// Left arrow: switch to PACKAGES tab.
		if m.mode == modeAgents {
			return m.enterInstalledMode()
		}
		return m, nil

	case key.Matches(msg, keyRight):
		// Right arrow: switch to AGENTS tab.
		if m.mode == modeInstalled {
			m.mode = modeAgents
			m.agentFilter = -1
			m.updateLayout()
			m.updateDetailContent()
			return m, nil
		}
		return m, nil

	case key.Matches(msg, keyDoctor):
		if m.mode == modeBrowse {
			return m, nil
		}
		m.mode = modeDoctor
		m.updateLayout()
		m.updateDetailContent()
		if !m.doctorLoaded {
			m.doctorLoaded = true
			m.statusBar.SetCenter("Running diagnostics...")
			return m, m.loadDoctor()
		}
		return m, nil

	case key.Matches(msg, keyTab):
		// Tab always switches focus to the detail pane (for scrolling).
		// Use p/a keys to switch between packages and agents tabs.
		if m.width >= minDetailWidth {
			m.focus = focusDetail
		}
		return m, nil

	case key.Matches(msg, keyEsc):
		if m.mode == modeBrowse {
			return m.exitBrowseMode()
		}
		if m.mode != modeInstalled {
			return m.enterInstalledMode()
		}
		return m, nil

	case key.Matches(msg, keyYank):
		return m.handleYank()

	case key.Matches(msg, keyEnter):
		return m.handleEnter()
	}

	// Delegate to the active list for navigation (j/k/up/down/etc.)
	prevIdx := m.activeListIndex()
	var cmd tea.Cmd
	switch m.mode {
	case modeInstalled, modeSearch:
		m.pkgList, cmd = m.pkgList.Update(msg)
	case modeAgents:
		m.agentList, cmd = m.agentList.Update(msg)
	case modeDoctor:
		m.doctorList, cmd = m.doctorList.Update(msg)
	case modeBrowse:
		m.fileList, cmd = m.fileList.Update(msg)
	}

	if m.activeListIndex() != prevIdx {
		m.updateDetailContent()
		if m.mode == modeBrowse {
			if item, ok := m.fileList.SelectedItem().(fileItem); ok && !item.isDir {
				dir := m.browseDir
				if dir == "" {
					dir = m.resolvePackageDir()
				}
				if dir != "" {
					return m, m.loadDirFileContent(dir, item.name)
				}
			}
		}
	}

	return m, cmd
}

func (m Model) handleSearchInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keyEsc):
		if len(m.pkgList.Items()) > 0 {
			m.focus = focusList
		} else {
			return m.enterInstalledMode()
		}
		return m, nil

	case key.Matches(msg, keyEnter):
		m.focus = focusList
		if m.searchQuery != "" {
			m.statusBar.SetCenter("Searching...")
			return m, m.searchPackages(m.searchQuery)
		}
		return m, nil

	case key.Matches(msg, keyTab):
		if len(m.pkgList.Items()) > 0 {
			m.focus = focusList
		}
		return m, nil

	case key.Matches(msg, keyForceQuit):
		m.quitting = true
		return m, tea.Quit

	default:
		s := msg.String()
		switch s {
		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			}
		case "ctrl+u":
			m.searchQuery = ""
		case "ctrl+w":
			// Delete last word.
			q := strings.TrimRight(m.searchQuery, " ")
			if idx := strings.LastIndex(q, " "); idx >= 0 {
				m.searchQuery = q[:idx+1]
			} else {
				m.searchQuery = ""
			}
		default:
			// Only accept printable single characters.
			if len(s) == 1 && s[0] >= 32 && s[0] < 127 {
				m.searchQuery += s
			}
		}
		return m, nil
	}
}

// ── Mode transitions ──

func (m Model) enterSearchMode() (tea.Model, tea.Cmd) {
	m.mode = modeSearch
	m.focus = focusSearch
	m.searchQuery = ""
	setCmd := m.pkgList.SetItems(nil)
	m.updateLayout()
	m.updateDetailContent()
	return m, setCmd
}

func (m Model) enterInstalledMode() (tea.Model, tea.Cmd) {
	m.mode = modeInstalled
	m.focus = focusList
	m.agentFilter = -1
	items := make([]list.Item, len(m.installed))
	for i, p := range m.installed {
		items[i] = p
	}
	setCmd := m.pkgList.SetItems(items)
	m.updateLayout()
	m.updateDetailContent()
	return m, setCmd
}

func (m Model) exitBrowseMode() (tea.Model, tea.Cmd) {
	// If we have a directory stack, go up one level.
	if len(m.browseDirStack) > 0 {
		parentDir := m.browseDirStack[len(m.browseDirStack)-1]
		m.browseDirStack = m.browseDirStack[:len(m.browseDirStack)-1]
		m.browseDir = parentDir
		return m, m.loadDirFiles(parentDir)
	}
	// No stack — return to origin mode.
	m.browseDirStack = nil
	m.browseDir = ""
	if m.browseOrigin == modeAgents {
		m.mode = modeAgents
		m.focus = focusList
		m.updateLayout()
		m.updateDetailContent()
		return m, nil
	}
	return m.enterInstalledMode()
}

// resolvePackageDir returns the current directory for the browsed package.
func (m *Model) resolvePackageDir() string {
	if m.browseDir != "" {
		return m.browseDir
	}
	// Find install path from installed cache.
	for _, p := range m.installed {
		if p.fullName == m.browsePackage {
			return p.installPath
		}
	}
	return ""
}

func (m Model) enterBrowseMode(item pkgItem) (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	m.focus = focusList
	m.browsePackage = item.fullName
	m.browseDir = item.installPath
	m.browseDirStack = nil
	m.browseOrigin = modeInstalled
	m.renderCache = make(map[string]string) // clear cache for new session
	m.updateLayout()
	return m, m.loadPackageFiles(item.fullName)
}

func (m Model) enterAgentBrowseMode(ag agentItem) (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	m.focus = focusList
	m.browsePackage = ag.name
	m.browseDir = ag.skillsDir
	m.browseDirStack = nil
	m.browseOrigin = modeAgents
	m.renderCache = make(map[string]string)
	m.updateLayout()
	return m, m.loadDirFiles(ag.skillsDir)
}

func (m Model) cycleAgentFilter(direction int) (tea.Model, tea.Cmd) {
	if len(m.agents) == 0 {
		return m, nil
	}
	// Cycle: -1 (All) → 0 → 1 → ... → len-1 → -1 (All)
	count := len(m.agents)
	m.agentFilter += direction
	if m.agentFilter >= count {
		m.agentFilter = -1
	} else if m.agentFilter < -1 {
		m.agentFilter = count - 1
	}

	// Re-filter the list.
	items := m.filterInstalledByAgent()
	cmd := m.pkgList.SetItems(items)
	m.updateDetailContent()
	return m, cmd
}

func (m *Model) filterInstalledByAgent() []list.Item {
	if m.agentFilter < 0 || m.agentFilter >= len(m.agents) {
		// All: return everything.
		items := make([]list.Item, len(m.installed))
		for i, p := range m.installed {
			items[i] = p
		}
		return items
	}

	agentName := m.agents[m.agentFilter].name
	var items []list.Item
	for _, p := range m.installed {
		state := m.getPackageState(p.fullName) // uses cache
		if state != nil && packageLinkedToAgent(state, agentName) {
			items = append(items, p)
		}
	}
	return items
}

// packageLinkedToAgent checks if a package has any skill or MCP link to the given agent.
func packageLinkedToAgent(state *installstate.PackageState, agentName string) bool {
	for _, s := range state.Skills {
		if strings.EqualFold(s.Agent, agentName) {
			return true
		}
	}
	for _, m := range state.MCP {
		if strings.EqualFold(m.Agent, agentName) {
			return true
		}
	}
	return false
}

// ── Action handlers ──

func (m Model) handleYank() (tea.Model, tea.Cmd) {
	var cmdStr string
	switch m.mode {
	case modeInstalled:
		if item, ok := m.pkgList.SelectedItem().(pkgItem); ok {
			cmdStr = "ctx info " + item.fullName
		}
	case modeSearch:
		if item, ok := m.pkgList.SelectedItem().(pkgItem); ok {
			cmdStr = "ctx install " + item.fullName
		}
	}
	if cmdStr != "" {
		m.statusBar.SetCenter("Copied: " + cmdStr)
		return m, tea.SetClipboard(cmdStr)
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeInstalled:
		if item, ok := m.pkgList.SelectedItem().(pkgItem); ok {
			if item.installed {
				return m.enterBrowseMode(item)
			}
			m.statusBar.SetCenter("→ ctx info " + item.fullName)
		}
	case modeAgents:
		// Enter on agent → browse agent's skills directory.
		idx := m.agentList.Index()
		if idx >= 0 && idx < len(m.agents) {
			ag := m.agents[idx]
			return m.enterAgentBrowseMode(ag)
		}
	case modeSearch:
		if item, ok := m.pkgList.SelectedItem().(pkgItem); ok {
			m.statusBar.SetCenter("→ ctx install " + item.fullName)
		}
	case modeBrowse:
		if item, ok := m.fileList.SelectedItem().(fileItem); ok {
			// ".." navigates up.
			if item.name == ".." {
				return m.exitBrowseMode()
			}
			if item.isDir {
				currentDir := m.browseDir
				if currentDir == "" {
					currentDir = m.resolvePackageDir()
				}
				m.browseDirStack = append(m.browseDirStack, currentDir)
				subDir := filepath.Join(currentDir, item.name)
				m.browseDir = subDir
				return m, m.loadDirFiles(subDir)
			}
			dir := m.browseDir
			if dir == "" {
				dir = m.resolvePackageDir()
			}
			return m, m.loadDirFileContent(dir, item.name)
		}
	}
	return m, nil
}

// ── Helpers ──

func (m *Model) activeListIndex() int {
	switch m.mode {
	case modeInstalled, modeSearch:
		return m.pkgList.Index()
	case modeAgents:
		return m.agentList.Index()
	case modeDoctor:
		return m.doctorList.Index()
	case modeBrowse:
		return m.fileList.Index()
	}
	return -1
}
