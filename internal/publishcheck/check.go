package publishcheck

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// CheckResult describes the validation result for one declared install method.
type CheckResult struct {
	Method string // brew, npm, pip, gem, cargo, script, source
	Pkg    string // the declared value
	OK     bool
	Error  string // human-readable error if !OK
}

// Check validates all declared install methods in the manifest.
// Only checks what the author declared — does not probe alternatives.
func Check(ctx context.Context, m *manifest.Manifest) []CheckResult {
	if m.Install == nil {
		return nil
	}

	var results []CheckResult
	spec := m.Install

	if spec.Brew != "" {
		results = append(results, checkCommand(ctx, "brew", spec.Brew, "brew", "info", spec.Brew))
	}
	if spec.Npm != "" {
		results = append(results, checkCommand(ctx, "npm", spec.Npm, "npm", "view", spec.Npm, "version"))
	}
	if spec.Pip != "" {
		results = append(results, checkCommand(ctx, "pip", spec.Pip, "pip3", "show", spec.Pip))
	}
	if spec.Gem != "" {
		results = append(results, checkCommand(ctx, "gem", spec.Gem, "gem", "specification", spec.Gem, "--remote"))
	}
	if spec.Cargo != "" {
		results = append(results, checkURL(ctx, "cargo", spec.Cargo,
			fmt.Sprintf("https://crates.io/api/v1/crates/%s", spec.Cargo)))
	}
	if spec.Script != "" {
		results = append(results, checkURL(ctx, "script", spec.Script, spec.Script))
	}
	if spec.Source != "" {
		if strings.HasPrefix(spec.Source, "https://") {
			// https:// sources are used as binary download URLs by FindAdapter;
			// validate reachability so broken links don't pass publish.
			results = append(results, checkURLFunc(ctx, "source", spec.Source, spec.Source))
		}
		// github:/npm:/pip: prefixes are declarative metadata, not direct
		// download URLs. Skip HTTP probing to avoid rate-limit failures.
	}

	return results
}

// checkCommand validates an install method by running a command.
// If the command binary isn't available locally, returns a skip (OK=true) with no error.
func checkCommand(ctx context.Context, method, pkg, bin string, args ...string) CheckResult {
	result := CheckResult{Method: method, Pkg: pkg}

	// If the package manager isn't installed locally, skip (not an error)
	if _, err := exec.LookPath(bin); err != nil {
		result.OK = true // can't verify, assume OK
		return result
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if err := cmd.Run(); err != nil {
		result.OK = false
		result.Error = fmt.Sprintf("%s not found via %s", pkg, method)
		return result
	}

	result.OK = true
	return result
}

// checkURLFunc is the default URL checker; tests may override it.
var checkURLFunc = checkURL

// checkURL validates an install method by doing an HTTP HEAD request.
func checkURL(ctx context.Context, method, pkg, url string) CheckResult {
	result := CheckResult{Method: method, Pkg: pkg}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		result.OK = false
		result.Error = fmt.Sprintf("invalid URL: %v", err)
		return result
	}
	req.Header.Set("User-Agent", "ctx-cli/publish-check")

	resp, err := client.Do(req)
	if err != nil {
		result.OK = false
		result.Error = fmt.Sprintf("unreachable: %v", err)
		return result
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 400 {
		result.OK = false
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	result.OK = true
	return result
}
