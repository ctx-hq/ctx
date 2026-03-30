package output

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Colors for terminal output.
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Dim    = "\033[2m"
)

// Success prints a success message.
func Success(format string, args ...any) {
	fmt.Fprintf(os.Stderr, Green+"✓ "+Reset+format+"\n", args...)
}

// Error prints an error message.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, Red+"✗ "+Reset+format+"\n", args...)
}

// Warn prints a warning message.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, Yellow+"! "+Reset+format+"\n", args...)
}

// Info prints an info message.
func Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, Cyan+"→ "+Reset+format+"\n", args...)
}

// Header prints a section header.
func Header(text string) {
	fmt.Fprintf(os.Stderr, "\n"+Bold+"%s"+Reset+"\n", text)
}

// Dim prints dimmed text.
func PrintDim(format string, args ...any) {
	fmt.Fprintf(os.Stderr, Dim+format+Reset+"\n", args...)
}

// Table prints a simple key-value table.
func Table(rows [][]string) {
	if len(rows) == 0 {
		return
	}
	maxKey := 0
	for _, row := range rows {
		if len(row) > 0 && len(row[0]) > maxKey {
			maxKey = len(row[0])
		}
	}
	for _, row := range rows {
		if len(row) >= 2 {
			_, _ = fmt.Fprintf(os.Stdout, "  %-*s  %s\n", maxKey, row[0], row[1])
		}
	}
}

// List prints a bulleted list.
func List(items []string) {
	for _, item := range items {
		_, _ = fmt.Fprintf(os.Stdout, "  • %s\n", item)
	}
}

// Separator prints a horizontal rule.
func Separator() {
	fmt.Fprintln(os.Stderr, Dim+strings.Repeat("─", 40)+Reset)
}

// Verbose prints a diagnostic message to stderr only when verbose mode is
// active in the given context. Use this from internal packages that don't
// have direct access to the Writer.
func Verbose(ctx context.Context, format string, args ...any) {
	if !IsVerboseContext(ctx) {
		return
	}
	fmt.Fprintf(os.Stderr, Dim+"[verbose] "+Reset+format+"\n", args...)
}
