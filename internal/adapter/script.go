package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const scriptMaxBytes = 10 * 1024 * 1024 // 10 MB

type scriptEnvKey struct{}

// WithScriptEnv attaches key=value environment variables to the context.
// These are passed to install scripts executed by the ScriptAdapter.
func WithScriptEnv(ctx context.Context, env []string) context.Context {
	return context.WithValue(ctx, scriptEnvKey{}, env)
}

func envFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(scriptEnvKey{}).([]string); ok {
		return v
	}
	return nil
}

// ScriptAdapterName is the canonical name for the script adapter.
const ScriptAdapterName = "script"

// ScriptAdapter installs CLI tools via shell scripts (curl|sh pattern).
type ScriptAdapter struct{}

func (a *ScriptAdapter) Name() string { return ScriptAdapterName }

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
	if extra := envFromContext(ctx); len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute script: %w", err)
	}

	return nil
}

// Uninstall removes the binary at the given absolute path.
// The path is expected to be state.CLI.BinaryPath (resolved during install).
// It refuses to remove binaries in system-protected directories.
func (a *ScriptAdapter) Uninstall(_ context.Context, binaryPath string) error {
	if binaryPath == "" {
		return fmt.Errorf("no binary path recorded; cannot clean up script-installed binary")
	}

	if !filepath.IsAbs(binaryPath) {
		return fmt.Errorf("binary path must be absolute, got %q", binaryPath)
	}

	// Refuse to touch system-protected directories (Unix-only;
	// ScriptAdapter.Available() already excludes Windows).
	for _, prefix := range []string{"/usr/bin/", "/bin/", "/sbin/", "/usr/sbin/"} {
		if strings.HasPrefix(binaryPath, prefix) {
			return fmt.Errorf("refusing to remove %s: system-protected directory", binaryPath)
		}
	}

	info, err := os.Lstat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already gone — idempotent
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to remove %s: is a directory", binaryPath)
	}

	return os.Remove(binaryPath)
}
