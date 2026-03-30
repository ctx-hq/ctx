package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompter abstracts interactive input for testability.
type Prompter interface {
	// Text prompts for a text value with a default.
	// Displays: "  label: (defaultVal) "
	// Returns defaultVal if user enters empty string.
	Text(label, defaultVal string) (string, error)

	// Confirm prompts for a yes/no with a default.
	// Displays: "  label [Y/n] " or "  label [y/N] "
	// Returns defaultVal if user enters empty string.
	Confirm(label string, defaultVal bool) (bool, error)

	// Select prompts the user to choose from a numbered list.
	// Displays each option with a 1-based index, returns the 0-based selected index.
	// Returns defaultIdx if user enters empty string.
	Select(label string, options []string, defaultIdx int) (int, error)
}

// TTYPrompter reads from a reader (typically os.Stdin).
type TTYPrompter struct {
	reader *bufio.Reader
}

// New creates a TTYPrompter from a reader.
func New(r *bufio.Reader) *TTYPrompter {
	return &TTYPrompter{reader: r}
}

// Text prompts for text input.
func (p *TTYPrompter) Text(label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "  %s: (%s) ", label, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "  %s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// Confirm prompts for yes/no.
func (p *TTYPrompter) Confirm(label string, defaultVal bool) (bool, error) {
	hint := "[Y/n]"
	if !defaultVal {
		hint = "[y/N]"
	}
	fmt.Fprintf(os.Stderr, "  %s %s ", label, hint)

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read input: %w", err)
	}

	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "":
		return defaultVal, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return defaultVal, nil
	}
}

// Select prompts the user to choose from a numbered list.
func (p *TTYPrompter) Select(label string, options []string, defaultIdx int) (int, error) {
	fmt.Fprintf(os.Stderr, "  %s:\n", label)
	for i, opt := range options {
		marker := "  "
		if i == defaultIdx {
			marker = "* "
		}
		fmt.Fprintf(os.Stderr, "    %s%d. %s\n", marker, i+1, opt)
	}
	fmt.Fprintf(os.Stderr, "  Choice [%d]: ", defaultIdx+1)

	line, err := p.reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read input: %w", err)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return defaultIdx, nil
	}

	var choice int
	if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(options) {
		fmt.Fprintf(os.Stderr, "  Invalid choice, using default (%d)\n", defaultIdx+1)
		return defaultIdx, nil
	}
	return choice - 1, nil
}

// NoopPrompter always returns defaults. Used for non-TTY, CI, and --yes flag.
type NoopPrompter struct{}

// Text returns the default value.
func (NoopPrompter) Text(_, defaultVal string) (string, error) {
	return defaultVal, nil
}

// Confirm returns the default value.
func (NoopPrompter) Confirm(_ string, defaultVal bool) (bool, error) {
	return defaultVal, nil
}

// Select returns the default index.
func (NoopPrompter) Select(_ string, _ []string, defaultIdx int) (int, error) {
	return defaultIdx, nil
}

// DefaultPrompter returns a TTYPrompter if stdin is a terminal, else NoopPrompter.
func DefaultPrompter() Prompter {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return New(bufio.NewReader(os.Stdin))
	}
	return NoopPrompter{}
}
