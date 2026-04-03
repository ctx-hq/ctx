package securityscan

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Severity indicates the severity of a security finding.
type Severity string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Medium   Severity = "medium"
)

// Finding represents a single security issue detected in a file.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Match    string   `json:"match"` // matched content snippet
}

// ScanResult holds the results of scanning a directory or file.
type ScanResult struct {
	Findings []Finding `json:"findings"`
	Scanned  int       `json:"files_scanned"`
}

// Passed returns true if no critical or high findings were detected.
func (r *ScanResult) Passed() bool {
	for _, f := range r.Findings {
		if f.Severity == Critical || f.Severity == High {
			return false
		}
	}
	return true
}

// HasCritical returns true if any critical findings exist.
func (r *ScanResult) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == Critical {
			return true
		}
	}
	return false
}

// rule defines a security detection pattern.
type rule struct {
	ID       string
	Pattern  *regexp.Regexp
	Severity Severity
	Message  string
}

// rules is the set of security detection patterns.
//
// Known limitations of line-based matching:
//   - Backslash-continued lines (e.g. "curl ... \\n| bash") are not detected.
//   - Indirect invocations via variables (e.g. CMD=curl; $CMD ... | bash) are not detected.
//   - Language-native evals without shell-style $ variables (e.g. Python eval(input())) are not detected.
var rules = []rule{
	// Remote code execution
	{ID: "RCE001", Pattern: regexp.MustCompile(`curl\s.*\|\s*(ba)?sh`), Severity: Critical, Message: "Remote code execution: curl piped to shell"},
	{ID: "RCE002", Pattern: regexp.MustCompile(`wget\s.*\|\s*(ba)?sh`), Severity: Critical, Message: "Remote code execution: wget piped to shell"},
	{ID: "RCE003", Pattern: regexp.MustCompile(`base64\s+(-d|--decode).*\|\s*(ba)?sh`), Severity: Critical, Message: "Obfuscated payload execution: base64 decode piped to shell"},

	// Code injection
	{ID: "INJ001", Pattern: regexp.MustCompile(`eval\s*\(.*\$`), Severity: High, Message: "Code injection: eval with variable interpolation"},
	{ID: "INJ002", Pattern: regexp.MustCompile(`exec\s*\(.*\$`), Severity: High, Message: "Code injection: exec with variable interpolation"},

	// Credential theft
	{ID: "CRED001", Pattern: regexp.MustCompile(`(cat|<)\s*~/\.ssh/`), Severity: High, Message: "SSH key access detected"},
	{ID: "CRED002", Pattern: regexp.MustCompile(`(^|\s)(printenv|/proc/self/environ)`), Severity: High, Message: "Environment variable enumeration detected"},

	// Reverse shell
	{ID: "SHELL001", Pattern: regexp.MustCompile(`/dev/tcp/`), Severity: Critical, Message: "Reverse shell: /dev/tcp connection"},
	{ID: "SHELL002", Pattern: regexp.MustCompile(`mkfifo.*\bnc\b`), Severity: Critical, Message: "Reverse shell: mkfifo with netcat"},

	// Data exfiltration
	{ID: "EXFIL001", Pattern: regexp.MustCompile(`curl.*-d.*\$\(`), Severity: High, Message: "Potential data exfiltration: curl POST with command substitution"},

	// Excessive permissions
	{ID: "PERM001", Pattern: regexp.MustCompile(`chmod\s+777`), Severity: Medium, Message: "Excessive file permissions: chmod 777"},
}

// scannableExts defines which file extensions are scanned.
var scannableExts = map[string]bool{
	".sh": true, ".bash": true, ".zsh": true,
	".py": true, ".rb": true, ".pl": true,
	".js": true, ".ts": true, ".mjs": true, ".cjs": true,
	".yaml": true, ".yml": true,
	".md": true, // SKILL.md is the main payload for skill packages
}

// skipDirs are directory names to skip during traversal.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "__pycache__": true,
	".venv": true, "vendor": true, ".next": true,
}

// maxFileSize is the maximum file size to scan (1MB).
const maxFileSize = 1 << 20

// Scan scans all eligible files in a directory for security issues.
func Scan(dir string) (*ScanResult, error) {
	result := &ScanResult{}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil // skip large files
		}

		// Check by extension first; if no extension, check for shebang
		if ext == "" {
			if !hasShebang(path) {
				return nil
			}
		} else if !scannableExts[ext] {
			return nil
		}

		findings, scanErr := ScanFile(path)
		if scanErr != nil {
			return nil // skip unreadable files
		}

		// Make paths relative to dir for cleaner output
		relPath, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			relPath = path
		}
		for i := range findings {
			findings[i].File = relPath
		}

		result.Findings = append(result.Findings, findings...)
		result.Scanned++
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan directory: %w", err)
	}

	return result, nil
}

// hasShebang returns true if the file starts with "#!" (script without extension).
func hasShebang(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 2)
	n, _ := f.Read(buf)
	return n == 2 && buf[0] == '#' && buf[1] == '!'
}

// matchLine checks a single line against all rules and returns any findings.
func matchLine(filename string, lineNum int, line string) []Finding {
	var findings []Finding
	for _, r := range rules {
		if loc := r.Pattern.FindStringIndex(line); loc != nil {
			match := line[loc[0]:loc[1]]
			if len(match) > 80 {
				match = match[:80] + "..."
			}
			findings = append(findings, Finding{
				File:     filename,
				Line:     lineNum,
				Rule:     r.ID,
				Severity: r.Severity,
				Message:  r.Message,
				Match:    match,
			})
		}
	}
	return findings
}

// ScanFile scans a single file for security issues.
func ScanFile(path string) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var findings []Finding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		findings = append(findings, matchLine(path, lineNum, scanner.Text())...)
	}

	return findings, scanner.Err()
}

// ScanContent scans a string of content for security issues.
func ScanContent(filename, content string) []Finding {
	var findings []Finding
	for lineNum, line := range strings.Split(content, "\n") {
		findings = append(findings, matchLine(filename, lineNum+1, line)...)
	}
	return findings
}
