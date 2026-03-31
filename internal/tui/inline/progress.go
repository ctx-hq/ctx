package inline

import (
	"context"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// progressMsg carries a progress update from the background function.
type progressMsg struct {
	percent float64
}

// progressDoneMsg signals that the background function has completed.
type progressDoneMsg struct {
	err error
}

// progressModel is the BubbleTea model for an inline progress display.
// prog is a shared pointer so the background goroutine can send messages
// back to the program even though BubbleTea copies the model by value.
type progressModel struct {
	label   string
	spinner spinner.Model
	percent float64
	done    bool
	err     error
	fn      func(ctx context.Context, report func(float64)) error
	prog    **tea.Program
	cancel  context.CancelFunc
}

func newProgressModel(label string, fn func(ctx context.Context, report func(float64)) error, cancel context.CancelFunc) progressModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	var p *tea.Program
	return progressModel{
		label:  label,
		spinner: s,
		fn:     fn,
		prog:   &p,
		cancel: cancel,
	}
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.runFn(),
	)
}

func (m progressModel) runFn() tea.Cmd {
	progPtr := m.prog
	return func() tea.Msg {
		report := func(pct float64) {
			if p := *progPtr; p != nil {
				p.Send(progressMsg{percent: pct})
			}
		}
		err := m.fn(context.Background(), report)
		return progressDoneMsg{err: err}
	}
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		m.percent = msg.percent
		return m, nil
	case progressDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = context.Canceled
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m progressModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	var display string
	if m.percent > 0 {
		// Show percentage when we have determinate progress.
		bar := renderBar(m.percent, 20)
		display = m.spinner.View() + " " + m.label + " " + bar
	} else {
		display = m.spinner.View() + " " + m.label
	}
	return tea.NewView(display)
}

// renderBar renders a simple text-based progress bar.
func renderBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	empty := width - filled
	bar := make([]byte, 0, width+2)
	bar = append(bar, '[')
	for i := 0; i < filled; i++ {
		bar = append(bar, '#')
	}
	for i := 0; i < empty; i++ {
		bar = append(bar, '.')
	}
	bar = append(bar, ']')
	return string(bar)
}

// RunWithProgress runs fn with an inline progress display.
// The provided ctx is passed to fn; Ctrl+C in the UI cancels it.
// Non-TTY: runs fn silently without any display.
func RunWithProgress(ctx context.Context, label string, fn func(ctx context.Context, report func(float64)) error) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fn(ctx, func(float64) {})
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wrappedFn := func(_ context.Context, report func(float64)) error {
		return fn(ctx, report)
	}

	model := newProgressModel(label, wrappedFn, cancel)
	prog := tea.NewProgram(model)
	*model.prog = prog

	final, err := prog.Run()
	if err != nil {
		return err
	}
	m := final.(progressModel)
	return m.err
}
