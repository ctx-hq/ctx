package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/term"
)

// Format represents the output format mode.
type Format int

const (
	FormatAuto     Format = iota // Auto-detect: TTY → Styled, pipe → JSON
	FormatJSON                   // Full JSON envelope with breadcrumbs
	FormatStyled                 // ANSI-colored human-readable output
	FormatQuiet                  // Raw JSON data only, no envelope
	FormatMarkdown               // GitHub-flavored Markdown tables
	FormatIDs                    // One ID per line
	FormatCount                  // Just the count of items
)

// Breadcrumb guides users/agents to the next logical action.
type Breadcrumb struct {
	Action      string `json:"action"`
	Command     string `json:"cmd"`
	Description string `json:"description"`
}

// Response is the standard JSON envelope for all command output.
type Response struct {
	OK          bool           `json:"ok"`
	Data        any            `json:"data,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Notice      string         `json:"notice,omitempty"`
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// ErrorResponse is the standard JSON envelope for errors.
type ErrorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Code  string `json:"code"`
	Hint  string `json:"hint,omitempty"`
}

// ResponseOption configures a Response before output.
type ResponseOption func(*Response)

// WithSummary sets a human-readable summary line.
func WithSummary(s string) ResponseOption {
	return func(r *Response) { r.Summary = s }
}

// WithNotice sets an informational notice (e.g., truncation info).
func WithNotice(s string) ResponseOption {
	return func(r *Response) { r.Notice = s }
}

// WithBreadcrumbs adds navigation hints for users/agents.
func WithBreadcrumbs(b ...Breadcrumb) ResponseOption {
	return func(r *Response) { r.Breadcrumbs = append(r.Breadcrumbs, b...) }
}

// WithMeta adds arbitrary metadata to the response.
func WithMeta(key string, value any) ResponseOption {
	return func(r *Response) {
		if r.Meta == nil {
			r.Meta = make(map[string]any)
		}
		r.Meta[key] = value
	}
}

// WriterOption configures a Writer.
type WriterOption func(*Writer)

// WithFormat sets the output format.
func WithFormat(f Format) WriterOption {
	return func(w *Writer) { w.format = f }
}

// WithStdout sets the stdout writer (default: os.Stdout).
func WithStdout(out io.Writer) WriterOption {
	return func(w *Writer) { w.stdout = out }
}

// WithStderr sets the stderr writer (default: os.Stderr).
func WithStderr(err io.Writer) WriterOption {
	return func(w *Writer) { w.stderr = err }
}

// Writer handles formatted output for all commands.
type Writer struct {
	format Format
	stdout io.Writer
	stderr io.Writer
}

// NewWriter creates a new Writer with the given options.
func NewWriter(opts ...WriterOption) *Writer {
	w := &Writer{
		format: FormatAuto,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// EffectiveFormat resolves FormatAuto to a concrete format based on TTY detection.
func (w *Writer) EffectiveFormat() Format {
	if w.format != FormatAuto {
		return w.format
	}
	if isTTY(w.stdout) {
		return FormatStyled
	}
	return FormatJSON
}

// IsStyled returns true if output will be human-readable styled.
func (w *Writer) IsStyled() bool {
	return w.EffectiveFormat() == FormatStyled
}

// IsMachine returns true if output is for machine consumption.
func (w *Writer) IsMachine() bool {
	f := w.EffectiveFormat()
	return f == FormatJSON || f == FormatQuiet || f == FormatIDs || f == FormatCount
}

// OK outputs a successful response in the configured format.
func (w *Writer) OK(data any, opts ...ResponseOption) error {
	resp := &Response{OK: true, Data: data}
	for _, opt := range opts {
		opt(resp)
	}

	switch w.EffectiveFormat() {
	case FormatJSON:
		return w.writeJSON(resp)
	case FormatQuiet:
		return w.writeQuiet(resp)
	case FormatStyled:
		return w.writeStyled(resp)
	case FormatMarkdown:
		return w.writeMarkdown(resp)
	case FormatIDs:
		return w.writeIDs(resp)
	case FormatCount:
		return w.writeCount(resp)
	default:
		return w.writeJSON(resp)
	}
}

// Err outputs an error in the configured format.
func (w *Writer) Err(err error) error {
	cliErr := AsCLIError(err)
	if cliErr == nil {
		cliErr = &CLIError{Code: CodeUsage, Message: err.Error()}
	}

	if w.IsMachine() {
		resp := ErrorResponse{
			OK:    false,
			Error: cliErr.Message,
			Code:  cliErr.Code,
			Hint:  cliErr.Hint,
		}
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	// Styled error output
	if _, err := fmt.Fprintf(w.stderr, Red+"✗ "+Reset+"%s\n", cliErr.Message); err != nil {
		return err
	}
	if cliErr.Hint != "" {
		if _, err := fmt.Fprintf(w.stderr, Dim+"  Hint: %s"+Reset+"\n", cliErr.Hint); err != nil {
			return err
		}
	}
	return nil
}

// --- Format implementations ---

func (w *Writer) writeJSON(resp *Response) error {
	enc := json.NewEncoder(w.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func (w *Writer) writeQuiet(resp *Response) error {
	enc := json.NewEncoder(w.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp.Data)
}

func (w *Writer) writeStyled(resp *Response) error {
	items := toSlice(resp.Data)
	if items == nil {
		// Single object — print as key-value table
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp.Data)
	}

	if len(items) == 0 {
		if resp.Summary != "" {
			_, _ = fmt.Fprintln(w.stderr, Dim+resp.Summary+Reset)
		}
		return nil
	}

	// Print items as formatted rows
	for _, item := range items {
		if m, ok := asMap(item); ok {
			w.printMapRow(m)
		} else {
			_, _ = fmt.Fprintf(w.stdout, "  %v\n", item)
		}
	}

	if resp.Summary != "" {
		_, _ = fmt.Fprintf(w.stderr, "\n"+Dim+"  %s"+Reset+"\n", resp.Summary)
	}
	if resp.Notice != "" {
		_, _ = fmt.Fprintf(w.stderr, Dim+"  %s"+Reset+"\n", resp.Notice)
	}
	return nil
}

func (w *Writer) writeMarkdown(resp *Response) error {
	items := toSlice(resp.Data)
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w.stdout, "*No data*")
		return nil
	}

	// Extract column headers from first item
	first, ok := asMap(items[0])
	if !ok {
		// Fallback to JSON
		return w.writeQuiet(resp)
	}

	headers := sortedKeys(first)
	// Header row
	_, _ = fmt.Fprintf(w.stdout, "| %s |\n", strings.Join(headers, " | "))
	// Separator
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	_, _ = fmt.Fprintf(w.stdout, "| %s |\n", strings.Join(seps, " | "))
	// Data rows
	for _, item := range items {
		if m, ok := asMap(item); ok {
			vals := make([]string, len(headers))
			for i, h := range headers {
				vals[i] = fmt.Sprintf("%v", m[h])
			}
			_, _ = fmt.Fprintf(w.stdout, "| %s |\n", strings.Join(vals, " | "))
		}
	}
	return nil
}

func (w *Writer) writeIDs(resp *Response) error {
	items := toSlice(resp.Data)
	for _, item := range items {
		if m, ok := asMap(item); ok {
			for _, key := range []string{"id", "full_name", "name"} {
				if v, exists := m[key]; exists {
					_, _ = fmt.Fprintf(w.stdout, "%v\n", v)
					break
				}
			}
		} else {
			_, _ = fmt.Fprintf(w.stdout, "%v\n", item)
		}
	}
	return nil
}

func (w *Writer) writeCount(resp *Response) error {
	items := toSlice(resp.Data)
	if items != nil {
		_, _ = fmt.Fprintf(w.stdout, "%d\n", len(items))
	} else if resp.Data != nil {
		_, _ = fmt.Fprintln(w.stdout, "1")
	} else {
		_, _ = fmt.Fprintln(w.stdout, "0")
	}
	return nil
}

// --- Helpers ---

func (w *Writer) printMapRow(m map[string]any) {
	// Try common patterns: full_name/name + type + version/description
	name := mapStr(m, "full_name", "name", "id")
	typ := mapStr(m, "type")
	extra := mapStr(m, "version", "description", "detail")

	if name != "" {
		typeBadge := ""
		if typ != "" {
			typeBadge = fmt.Sprintf(" [%s]", typ)
		}
		_, _ = fmt.Fprintf(w.stdout, "  %-30s%-8s %s\n", name, typeBadge, extra)
	} else {
		// Fallback: print all keys
		parts := make([]string, 0)
		for _, k := range sortedKeys(m) {
			parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
		}
		_, _ = fmt.Fprintf(w.stdout, "  %s\n", strings.Join(parts, " "))
	}
}

// toSlice converts data to a []any if it's a slice type, or nil otherwise.
func toSlice(data any) []any {
	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice {
		result := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = v.Index(i).Interface()
		}
		return result
	}
	return nil
}

// asMap converts a value to map[string]any. Works for maps directly and for
// structs via JSON roundtrip (so json tags are respected).
func asMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

// mapStr returns the first non-empty string value for the given keys.
func mapStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isTTY checks if a writer is connected to a terminal.
func isTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// ResolveFormat determines the output format from flag values.
// Returns an error if multiple mutually exclusive flags are set.
func ResolveFormat(jsonFlag, quietFlag, styledFlag, mdFlag, idsFlag, countFlag, agentFlag bool) (Format, error) {
	count := 0
	if jsonFlag {
		count++
	}
	if quietFlag {
		count++
	}
	if styledFlag {
		count++
	}
	if mdFlag {
		count++
	}
	if idsFlag {
		count++
	}
	if countFlag {
		count++
	}
	if agentFlag {
		count++
	}
	if count > 1 {
		return FormatAuto, ErrUsage("output format flags are mutually exclusive: --json, --quiet, --styled, --md, --ids-only, --count, --agent")
	}

	switch {
	case jsonFlag:
		return FormatJSON, nil
	case quietFlag:
		return FormatQuiet, nil
	case styledFlag:
		return FormatStyled, nil
	case mdFlag:
		return FormatMarkdown, nil
	case idsFlag:
		return FormatIDs, nil
	case countFlag:
		return FormatCount, nil
	case agentFlag:
		return FormatQuiet, nil // agent mode = quiet + JSON errors
	default:
		return FormatAuto, nil
	}
}
