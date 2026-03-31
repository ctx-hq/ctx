package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/tui"
)

var (
	doctorKeyEsc = key.NewBinding(key.WithKeys("esc"))
	doctorKeyD   = key.NewBinding(key.WithKeys("d"))
)

// doctorOverlay shows diagnostic check results in a centered panel.
type doctorOverlay struct {
	viewport viewport.Model
	visible  bool
	loading  bool
	width    int
	height   int
}

func newDoctorOverlay(width, height int) doctorOverlay {
	vp := viewport.New(
		viewport.WithWidth(width - 4),
		viewport.WithHeight(height - 4),
	)
	return doctorOverlay{
		viewport: vp,
		width:    width,
		height:   height,
	}
}

// Toggle toggles the overlay visibility. Returns a command if loading is needed.
func (o *doctorOverlay) Toggle(svc Service) tea.Cmd {
	o.visible = !o.visible
	if o.visible {
		o.loading = true
		return loadDoctorChecks(svc)
	}
	return nil
}

func loadDoctorChecks(svc Service) tea.Cmd {
	return func() tea.Msg {
		result := svc.RunDoctorChecks()
		return doctorResultMsg{Result: result}
	}
}

func (o doctorOverlay) Update(msg tea.Msg) (doctorOverlay, tea.Cmd) {
	if !o.visible {
		return o, nil
	}

	switch msg := msg.(type) {
	case doctorResultMsg:
		o.loading = false
		o.viewport.SetContent(formatDoctorResult(msg.Result))
		return o, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, doctorKeyEsc) || key.Matches(msg, doctorKeyD) {
			o.visible = false
			return o, nil
		}
	}

	var cmd tea.Cmd
	o.viewport, cmd = o.viewport.Update(msg)
	return o, cmd
}

func (o doctorOverlay) View() string {
	if !o.visible {
		return ""
	}

	var content string
	if o.loading {
		content = "  Running diagnostic checks..."
	} else {
		content = o.viewport.View()
	}

	bordered := tui.DetailBorder.
		Width(o.width - 4).
		Height(o.height - 4).
		Render(content)

	return lipgloss.Place(o.width, o.height, lipgloss.Center, lipgloss.Center, bordered)
}

// SetSize updates the overlay dimensions.
func (o *doctorOverlay) SetSize(w, h int) {
	o.width = w
	o.height = h
	o.viewport.SetWidth(w - 6)
	o.viewport.SetHeight(h - 6)
}

// formatDoctorResult formats doctor check results for display.
func formatDoctorResult(r *doctor.Result) string {
	if r == nil {
		return "  No results"
	}

	var b strings.Builder
	b.WriteString(tui.ListItemTitle.Render("Doctor Checks"))
	b.WriteString("\n\n")

	for _, c := range r.Checks {
		var icon string
		switch c.Status {
		case "pass":
			icon = tui.SuccessStyle.Render(tui.IconPass)
		case "warn":
			icon = tui.WarningStyle.Render(tui.IconWarn)
		case "fail":
			icon = tui.ErrorStyle.Render(tui.IconFail)
		default:
			icon = "?"
		}
		b.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, c.Name, c.Detail))
		if c.Hint != "" {
			b.WriteString(fmt.Sprintf("    %s\n", tui.HelpStyle.Render(c.Hint)))
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", r.Summary()))

	return b.String()
}
