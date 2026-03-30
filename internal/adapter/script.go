package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const scriptMaxBytes = 10 * 1024 * 1024 // 10 MB

// ScriptAdapter installs CLI tools via shell scripts (curl|sh pattern).
type ScriptAdapter struct{}

func (a *ScriptAdapter) Name() string { return "script" }

func (a *ScriptAdapter) Available() bool {
	return runtime.GOOS != "windows"
}

func (a *ScriptAdapter) Install(ctx context.Context, scriptURL string) error {
	if !strings.HasPrefix(scriptURL, "https://") {
		return fmt.Errorf("script URL must use https://, got %q", scriptURL)
	}

	// Download script to temp file (safer than piping directly to shell)
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect to non-HTTPS URL: %s", req.URL)
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download script: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download script: status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "ctx-script-*.sh")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, scriptMaxBytes)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write script: %w", err)
	}
	_ = tmp.Close()

	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return fmt.Errorf("chmod script: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute script: %w", err)
	}

	return nil
}

func (a *ScriptAdapter) Uninstall(_ context.Context, _ string) error {
	return fmt.Errorf("script-installed packages must be removed manually")
}
