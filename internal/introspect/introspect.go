package introspect

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

var semverExtract = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

// CaptureHelp runs "<binary> --help" and returns the output.
// Falls back to "<binary> -h" if --help fails.
// Many CLIs return non-zero exit codes for --help, so we accept output
// regardless of exit code as long as there's meaningful content.
func CaptureHelp(binary string) (string, error) {
	out, _ := exec.Command(binary, "--help").CombinedOutput()
	if len(out) > 0 {
		return string(out), nil
	}
	out, _ = exec.Command(binary, "-h").CombinedOutput()
	if len(out) > 0 {
		return string(out), nil
	}
	out, _ = exec.Command(binary, "help").CombinedOutput()
	if len(out) > 0 {
		return string(out), nil
	}
	return "", fmt.Errorf("could not capture help for %s", binary)
}

// VersionResult holds the detected version and the command that produced it.
type VersionResult struct {
	Version string // semver string, e.g. "1.22.0"
	Command string // the flag/subcommand that worked, e.g. "--version" or "version"
}

// CaptureVersion runs "<binary> --version" and extracts a semver string.
// Returns version "0.1.0" with command "--version" if no version can be parsed.
// Like CaptureHelp, we ignore exit codes and check for meaningful output,
// since many CLIs return non-zero for --version.
func CaptureVersion(binary string) VersionResult {
	for _, arg := range []string{"--version", "version"} {
		out, _ := exec.Command(binary, arg).CombinedOutput()
		if m := semverExtract.FindString(string(out)); m != "" {
			return VersionResult{Version: m, Command: arg}
		}
	}
	return VersionResult{Version: "0.1.0", Command: "--version"}
}

// DetectInstallMethod probes how a binary was installed on the current system.
// Returns an InstallSpec with the detected method populated.
func DetectInstallMethod(binary string) *manifest.InstallSpec {
	spec := &manifest.InstallSpec{}

	// Check brew
	if commandExists("brew") {
		out, err := exec.Command("brew", "list", "--formula").CombinedOutput()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) == binary {
					spec.Brew = binary
					return spec
				}
			}
		}
	}

	// Check npm global
	if commandExists("npm") {
		out, err := exec.Command("npm", "list", "-g", "--depth=0", "--json").CombinedOutput()
		if err == nil && strings.Contains(string(out), `"`+binary+`"`) {
			spec.Npm = binary
			return spec
		}
	}

	// Check pip
	if commandExists("pip3") {
		if err := exec.Command("pip3", "show", binary).Run(); err == nil {
			spec.Pip = binary
			return spec
		}
	}

	// Check cargo
	if commandExists("cargo") {
		out, err := exec.Command("cargo", "install", "--list").CombinedOutput()
		if err == nil && strings.Contains(string(out), binary) {
			spec.Cargo = binary
			return spec
		}
	}

	return spec
}

// GenerateSkillMD creates a basic SKILL.md from CLI name and help output.
func GenerateSkillMD(name, description, helpText string) string {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	if description != "" {
		b.WriteString("description: |\n")
		b.WriteString("  " + description + "\n")
	}
	b.WriteString("triggers:\n")
	b.WriteString("  - " + name + "\n")
	b.WriteString("  - /" + name + "\n")
	b.WriteString("---\n\n")

	b.WriteString("# " + name + "\n\n")

	if description != "" {
		b.WriteString(description + "\n\n")
	}

	b.WriteString("## Usage\n\n")
	b.WriteString("Run `" + name + "` commands via the terminal.\n\n")

	if helpText != "" {
		// Truncate excessively long help text
		if len(helpText) > 4000 {
			helpText = helpText[:4000] + "\n... (truncated)"
		}
		b.WriteString("## Reference\n\n")
		b.WriteString("```\n")
		b.WriteString(helpText)
		if !strings.HasSuffix(helpText, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}

	return b.String()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
