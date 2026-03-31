// Package inline provides BubbleTea-based inline TUI components for
// interactive prompts, progress displays, and agent selection.
package inline

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/tui"
)

// Compile-time check that BubblePrompter satisfies prompt.Prompter.
var _ prompt.Prompter = (*BubblePrompter)(nil)

// BubblePrompter implements prompt.Prompter using inline BubbleTea programs.
type BubblePrompter struct{}

// NewBubblePrompter creates a new BubblePrompter.
func NewBubblePrompter() *BubblePrompter { return &BubblePrompter{} }

// ---------------------------------------------------------------------------
// Text
// ---------------------------------------------------------------------------

// textModel is the BubbleTea model for a single-line text prompt.
type textModel struct {
	label      string
	defaultVal string
	input      textinput.Model
	submitted  bool
	cancelled  bool
}

func newTextModel(label, defaultVal string) textModel {
	ti := textinput.New()
	ti.Prompt = "  " + label + ": "
	ti.Placeholder = defaultVal
	ti.Focus()
	return textModel{
		label:      label,
		defaultVal: defaultVal,
		input:      ti,
	}
}

func (m textModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.submitted = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m textModel) View() tea.View {
	if m.submitted || m.cancelled {
		return tea.NewView("")
	}
	return tea.NewView(m.input.View())
}

// result returns the user's input value, falling back to defaultVal.
func (m textModel) result() string {
	v := strings.TrimSpace(m.input.Value())
	if v == "" {
		return m.defaultVal
	}
	return v
}

// Text prompts for a text value with a default using an inline BubbleTea program.
func (p *BubblePrompter) Text(label, defaultVal string) (string, error) {
	model := newTextModel(label, defaultVal)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return "", fmt.Errorf("text prompt: %w", err)
	}
	m := final.(textModel)
	if m.cancelled {
		return "", fmt.Errorf("cancelled")
	}
	return m.result(), nil
}

// ---------------------------------------------------------------------------
// Confirm
// ---------------------------------------------------------------------------

// confirmModel is the BubbleTea model for a Y/n confirmation prompt.
type confirmModel struct {
	label      string
	defaultVal bool
	input      textinput.Model
	submitted  bool
	cancelled  bool
}

func newConfirmModel(label string, defaultVal bool) confirmModel {
	hint := "[Y/n]"
	if !defaultVal {
		hint = "[y/N]"
	}
	ti := textinput.New()
	ti.Prompt = "  " + label + " " + hint + " "
	ti.CharLimit = 3
	ti.Focus()
	return confirmModel{
		label:      label,
		defaultVal: defaultVal,
		input:      ti,
	}
}

func (m confirmModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.submitted = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m confirmModel) View() tea.View {
	if m.submitted || m.cancelled {
		return tea.NewView("")
	}
	return tea.NewView(m.input.View())
}

// result interprets the user's input as a boolean, falling back to defaultVal.
func (m confirmModel) result() bool {
	v := strings.TrimSpace(strings.ToLower(m.input.Value()))
	switch v {
	case "":
		return m.defaultVal
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return m.defaultVal
	}
}

// Confirm prompts for a yes/no answer using an inline BubbleTea program.
func (p *BubblePrompter) Confirm(label string, defaultVal bool) (bool, error) {
	model := newConfirmModel(label, defaultVal)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return false, fmt.Errorf("confirm prompt: %w", err)
	}
	m := final.(confirmModel)
	if m.cancelled {
		return false, fmt.Errorf("cancelled")
	}
	return m.result(), nil
}

// ---------------------------------------------------------------------------
// Select
// ---------------------------------------------------------------------------

// Select key bindings.
var (
	selKeyDown    = key.NewBinding(key.WithKeys("j", "down"))
	selKeyUp      = key.NewBinding(key.WithKeys("k", "up"))
	selKeyConfirm = key.NewBinding(key.WithKeys("enter"))
)

// selectModel is the BubbleTea model for a numbered list selection prompt.
type selectModel struct {
	label     string
	options   []string
	cursor    int
	submitted bool
	cancelled bool
}

func newSelectModel(label string, options []string, defaultIdx int) selectModel {
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	return selectModel{
		label:   label,
		options: options,
		cursor:  defaultIdx,
	}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	n := len(m.options)
	if n == 0 {
		return m, tea.Quit
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, selKeyDown):
			m.cursor = (m.cursor + 1) % n
		case key.Matches(msg, selKeyUp):
			m.cursor = (m.cursor - 1 + n) % n
		case key.Matches(msg, selKeyConfirm):
			m.submitted = true
			return m, tea.Quit
		case msg.String() == "ctrl+c" || msg.String() == "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() tea.View {
	if m.submitted || m.cancelled {
		return tea.NewView("")
	}
	var b strings.Builder
	b.WriteString("  " + m.label + ":\n")
	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		label := fmt.Sprintf("%d. %s", i+1, opt)
		if i == m.cursor {
			label = tui.ListItemTitle.Render(label)
		}
		b.WriteString("    " + cursor + label + "\n")
	}
	b.WriteString(tui.HelpStyle.Render("  ↑/↓ navigate • enter select"))
	return tea.NewView(b.String())
}

// Select prompts the user to choose from a numbered list using an inline BubbleTea program.
func (p *BubblePrompter) Select(label string, options []string, defaultIdx int) (int, error) {
	model := newSelectModel(label, options, defaultIdx)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return 0, fmt.Errorf("select prompt: %w", err)
	}
	m := final.(selectModel)
	if m.cancelled {
		return 0, fmt.Errorf("cancelled")
	}
	return m.cursor, nil
}
