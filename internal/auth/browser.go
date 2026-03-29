package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// execCommand is the function used to create exec.Cmd. Replaceable in tests.
var execCommand = exec.Command

// OpenBrowser opens the specified URL in the user's default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return execCommand("open", url).Start()
	case "linux":
		return execCommand("xdg-open", url).Start()
	case "windows":
		// The empty "" is required as the title argument for cmd /c start,
		// otherwise special characters in the URL (like &) are misinterpreted.
		return execCommand("cmd", "/c", "start", "", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
