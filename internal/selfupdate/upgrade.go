package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Upgrade downloads and installs the specified version of ctx,
// replacing the current binary in-place. It delegates to the official
// install script which handles platform detection and checksum verification.
func Upgrade(version string) error {
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
			fmt.Sprintf("CTX_VERSION=%s CTX_INSTALL_DIR=%s CTX_QUIET=1 curl -fsSL https://getctx.org/install.sh | sh",
				version, installDir))
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "windows":
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("$env:CTX_VERSION='%s'; $env:CTX_INSTALL_DIR='%s'; irm https://getctx.org/install.ps1 | iex",
				version, installDir))
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
