package doctor

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/adapter"
	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/profile"
)

// Check represents a single diagnostic check result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "warn", "fail"
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// Result holds the complete diagnostic output.
type Result struct {
	Checks    []Check `json:"checks"`
	PassCount int     `json:"pass_count"`
	WarnCount int     `json:"warn_count"`
	FailCount int     `json:"fail_count"`
}

// Summary returns a human-readable summary of the diagnostic result.
func (r *Result) Summary() string {
	s := fmt.Sprintf("%d checks: %d pass", len(r.Checks), r.PassCount)
	if r.WarnCount > 0 {
		s += fmt.Sprintf(", %d warn", r.WarnCount)
	}
	if r.FailCount > 0 {
		s += fmt.Sprintf(", %d fail", r.FailCount)
	}
	return s
}

// RunChecks executes all diagnostic checks and returns the result.
// version is the ctx binary version string. token is the auth token (may be empty).
func RunChecks(version, token string) *Result {
	var checks []Check

	add := func(name, status, detail string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail})
	}
	addHint := func(name, status, detail, hint string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail, Hint: hint})
	}

	// 1. Version
	add("ctx version", "pass", fmt.Sprintf("%s (%s/%s)", version, runtime.GOOS, runtime.GOARCH))

	// 2. Config
	cfg, err := config.Load()
	if err != nil {
		addHint("config", "fail", err.Error(), "Run 'ctx init' or check ~/.ctx/config.yaml")
	} else {
		add("config", "pass", config.ConfigFilePath())
	}

	// 3. Auth
	if cfg != nil && token != "" {
		username := ""
		if res, err := profile.Resolve(""); err == nil {
			username = res.Profile.Username
		}
		if username != "" {
			add("auth", "pass", fmt.Sprintf("logged in as %s (profile: %s)", username, resolvedProfileName()))
		} else {
			add("auth", "pass", "logged in")
		}
	} else {
		addHint("auth", "warn", "not logged in", "Run 'ctx login' to authenticate")
	}

	// 4. Registry connectivity — use profile-resolved registry for consistency.
	if cfg != nil {
		registryURL := resolvedRegistryURL()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(registryURL + "/v1/health")
		if err != nil {
			addHint("registry", "fail",
				fmt.Sprintf("cannot reach %s", registryURL),
				"Check your internet connection")
		} else {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				add("registry", "pass", registryURL)
			} else {
				add("registry", "warn", fmt.Sprintf("%s returned %d", registryURL, resp.StatusCode))
			}
		}
	}

	// 5. Installed packages
	inst := installer.NewScanner()
	installed, scanErr := inst.ScanInstalled()
	if scanErr != nil {
		addHint("installed packages", "fail", scanErr.Error(), "Check ~/.ctx/packages/ directory")
	} else {
		add("installed packages", "pass", fmt.Sprintf("%d packages", len(installed)))
	}

	// 6. Detected agents
	agents := agent.DetectAll()
	if len(agents) > 0 {
		names := ""
		for i, a := range agents {
			if i > 0 {
				names += ", "
			}
			names += a.Name()
		}
		add("agents", "pass", names)
	} else {
		addHint("agents", "warn", "none detected", "Install Claude Code, Cursor, or Windsurf")
	}

	// 7. Links integrity (links.json)
	links, linkErr := installer.LoadLinks()
	if linkErr != nil {
		addHint("links registry", "warn", "cannot load links.json", "Will be created on next install")
	} else {
		issues := links.Verify()
		if len(issues) == 0 {
			totalLinks := 0
			for _, entries := range links.Links {
				totalLinks += len(entries)
			}
			add("links integrity", "pass", fmt.Sprintf("%d links, all intact", totalLinks))
		} else {
			detail := fmt.Sprintf("%d issue(s) found", len(issues))
			for i, issue := range issues {
				if i >= 3 {
					detail += fmt.Sprintf(" (+%d more)", len(issues)-3)
					break
				}
				detail += fmt.Sprintf("\n    %s: %s → %s (%s)", issue.Package, issue.Entry.Agent, issue.Entry.Target, issue.Problem)
			}
			addHint("links integrity", "warn", detail, "Run 'ctx ln --repair' or reinstall affected packages")
		}
	}

	// 8. Version store consistency (current symlinks)
	if scanErr == nil {
		brokenCurrent := 0
		checker := installer.NewScanner()
		for _, entry := range installed {
			cur := checker.CurrentVersion(entry.FullName)
			if cur == "" {
				brokenCurrent++
			}
		}
		if brokenCurrent == 0 {
			add("version store", "pass", "all current symlinks valid")
		} else {
			addHint("version store", "warn",
				fmt.Sprintf("%d package(s) missing current symlink", brokenCurrent),
				"Run 'ctx use <pkg>@<version>' to repair")
		}
	}

	// 9. Per-package health (via state.json)
	if scanErr == nil {
		var unhealthyDetails []string
		checker2 := installer.NewScanner()
		for _, entry := range installed {
			pkgDir := checker2.PackageDir(entry.FullName)
			state, _ := installstate.Load(pkgDir)
			if state == nil {
				continue
			}
			if state.CLI != nil && state.CLI.Status == "ok" {
				if err := adapter.Verify(state.CLI.Binary, ""); err != nil {
					unhealthyDetails = append(unhealthyDetails,
						fmt.Sprintf("%s: CLI binary %s not found", entry.FullName, state.CLI.Binary))
				}
			}
			for _, s := range state.Skills {
				if _, err := os.Stat(s.SymlinkPath); err != nil {
					unhealthyDetails = append(unhealthyDetails,
						fmt.Sprintf("%s: skill link broken for %s", entry.FullName, s.Agent))
				}
			}
		}
		if len(unhealthyDetails) == 0 {
			add("package health", "pass", "all components healthy")
		} else {
			detail := fmt.Sprintf("%d issue(s)", len(unhealthyDetails))
			for i, d := range unhealthyDetails {
				if i >= 3 {
					detail += fmt.Sprintf(" (+%d more)", len(unhealthyDetails)-3)
					break
				}
				detail += "\n    " + d
			}
			addHint("package health", "warn", detail, "Run 'ctx install <pkg>' to repair")
		}
	}

	// 10. Available adapters (package managers)
	for _, pm := range []struct{ name, cmd string }{
		{"brew", "brew"},
		{"npm", "npm"},
		{"pip", "pip3"},
		{"cargo", "cargo"},
		{"git", "git"},
	} {
		if _, err := exec.LookPath(pm.cmd); err == nil {
			out, _ := exec.Command(pm.cmd, "--version").Output()
			ver := strings.TrimSpace(string(out))
			if len(ver) > 40 {
				ver = ver[:40]
			}
			add(pm.name, "pass", ver)
		}
	}

	// Count results
	result := &Result{Checks: checks}
	for _, c := range checks {
		switch c.Status {
		case "pass":
			result.PassCount++
		case "warn":
			result.WarnCount++
		case "fail":
			result.FailCount++
		}
	}

	return result
}

// resolvedProfileName returns the resolved profile name, or "default".
func resolvedProfileName() string {
	res, err := profile.Resolve("")
	if err != nil {
		return "default"
	}
	return res.Name
}

// resolvedRegistryURL returns the registry URL from the resolved profile,
// falling back to config, then the default.
func resolvedRegistryURL() string {
	res, err := profile.Resolve("")
	if err == nil && res.Profile.Registry != "" {
		return res.Profile.Registry
	}
	cfg, cfgErr := config.Load()
	if cfgErr == nil {
		return cfg.RegistryURL()
	}
	return config.DefaultRegistry
}
