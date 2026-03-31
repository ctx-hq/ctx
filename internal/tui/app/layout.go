package app

import "github.com/ctx-hq/ctx/internal/installstate"

// Layout constants.
const (
	headerHeight   = 1
	footerHeight   = 1
	minDetailWidth = 60
)

func (m *Model) updateLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	contentHeight := m.height - headerHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	m.statusBar.SetWidth(m.width)

	lw := m.getListWidth()
	listHeight := contentHeight
	if m.mode == modeSearch {
		listHeight -= 2 // search input + blank line
	}
	if listHeight < 3 {
		listHeight = 3
	}

	// Set size for ALL lists so they're ready when mode switches.
	m.pkgList.SetSize(lw, listHeight)
	m.agentList.SetSize(lw, listHeight)
	m.doctorList.SetSize(lw, listHeight)
	m.fileList.SetSize(lw, listHeight)

	if m.width >= minDetailWidth {
		dw := m.detailWidth()
		m.detail.SetWidth(dw)
		m.detail.SetHeight(contentHeight)
	}
}

func (m *Model) getListWidth() int {
	if m.width >= minDetailWidth {
		w := m.width * 3 / 10
		if w < 25 {
			w = 25
		}
		if w > 50 {
			w = 50
		}
		return w
	}
	return m.width
}

// detailWidth returns the usable width for detail content.
func (m *Model) detailWidth() int {
	dw := m.width - m.getListWidth() - 5 // borders + padding + separator
	if dw < 10 {
		dw = 10
	}
	return dw
}

func (m *Model) updateDetailContent() {
	dw := m.detailWidth()

	var content string
	switch m.mode {
	case modeInstalled, modeSearch:
		if item, ok := m.pkgList.SelectedItem().(pkgItem); ok {
			var state *installstate.PackageState
			var skillContent string
			if item.installed {
				state = m.getPackageState(item.fullName)
				skillContent = m.getSkillContent(item.fullName)
			}
			content = renderPkgDetail(item, dw, state, skillContent)
		} else {
			content = renderEmptyDetail(m.mode, dw)
		}
	case modeAgents:
		if item, ok := m.agentList.SelectedItem().(agentItem); ok {
			var skills []AgentSkillEntry
			var mcpServers []AgentMCPEntry
			if m.service != nil {
				skills, mcpServers = m.service.GetAgentDetail(item.name)
			}
			content = renderAgentDetail(item, dw, skills, mcpServers)
		} else {
			content = renderEmptyDetail(modeAgents, dw)
		}
	case modeDoctor:
		if item, ok := m.doctorList.SelectedItem().(doctorItem); ok {
			content = renderDoctorDetail(item, dw)
		} else {
			content = renderEmptyDetail(modeDoctor, dw)
		}
	case modeBrowse:
		if item, ok := m.fileList.SelectedItem().(fileItem); ok {
			if item.isDir {
				content = detailTitle.Render("📁 "+item.name+"/") + "\n\n" + detailDim.Render("Enter to open directory")
			} else {
				content = detailTitle.Render(item.name) + "\n\n" + detailDim.Render("Loading...")
			}
		} else {
			content = renderEmptyDetail(modeBrowse, dw)
		}
	}

	m.detail.SetContent(content)
	m.detail.GotoTop()
}
