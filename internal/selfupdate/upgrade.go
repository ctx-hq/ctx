package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

// semverRe validates a strict semver version string (e.g., "1.2.3" or "1.2.3-beta").
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

// Upgrade downloads and installs the specified version of ctx,
// replacing the current binary in-place. It delegates to the official
// install script which handles platform detection and checksum verification.
func Upgrade(version string) error {
	// Validate version to prevent command injection
	if !semverRe.MatchString(version) {
		return fmt.Errorf("invalid version format: %q", version)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	installDir := filepath.Dir(exe)

	switch runtime.GOOS {
	case "darwin", "linux":
		cmd := exec.Command("sh", "-c",
			"curl -fsSL https://getctx.org/install.sh | sh")
		cmd.Env = append(os.Environ(),
			"CTX_VERSION="+version,
			"CTX_INSTALL_DIR="+installDir,
			"CTX_QUIET=1",
		)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "windows":
		cmd := exec.Command("powershell", "-Command",
			"irm https://getctx.org/install.ps1 | iex")
		cmd.Env = append(os.Environ(),
			"CTX_VERSION="+version,
			"CTX_INSTALL_DIR="+installDir,
		)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
