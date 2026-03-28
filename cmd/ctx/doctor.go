package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/getctx/ctx/internal/agent"
	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/output"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"dr"},
	Short:   "Diagnose environment and connectivity",
	Long: `Run diagnostic checks to verify your ctx installation,
configuration, network connectivity, and detected agents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		type Check struct {
			Name   string `json:"name"`
			Status string `json:"status"` // "pass", "warn", "fail"
			Detail string `json:"detail,omitempty"`
			Hint   string `json:"hint,omitempty"`
		}
		var checks []Check

		add := func(name, status, detail string) {
			checks = append(checks, Check{Name: name, Status: status, Detail: detail})
		}
		addHint := func(name, status, detail, hint string) {
			checks = append(checks, Check{Name: name, Status: status, Detail: detail, Hint: hint})
		}

		// 1. Version
		add("ctx version", "pass", fmt.Sprintf("%s (%s/%s)", Version, runtime.GOOS, runtime.GOARCH))

		// 2. Config
		cfg, err := config.Load()
		if err != nil {
			addHint("config", "fail", err.Error(), "Run 'ctx init' or check ~/.ctx/config.yaml")
		} else {
			add("config", "pass", config.ConfigFilePath())
		}

		// 3. Auth
		if cfg != nil && cfg.IsLoggedIn() {
			add("auth", "pass", fmt.Sprintf("logged in as %s", cfg.Username))
		} else {
			addHint("auth", "warn", "not logged in", "Run 'ctx login' to authenticate")
		}

		// 4. Registry connectivity
		if cfg != nil {
			registryURL := cfg.RegistryURL()
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(registryURL + "/v1/health")
			if err != nil {
				addHint("registry", "fail",
					fmt.Sprintf("cannot reach %s", registryURL),
					"Check your internet connection")
			} else {
				resp.Body.Close()
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
			checker := &installer.Installer{DataDir: config.DataDir()}
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

		// 9. Available adapters (package managers)
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
		passCount, warnCount, failCount := 0, 0, 0
		for _, c := range checks {
			switch c.Status {
			case "pass":
				passCount++
			case "warn":
				warnCount++
			case "fail":
				failCount++
			}
		}
		summary := fmt.Sprintf("%d checks: %d pass", len(checks), passCount)
		if warnCount > 0 {
			summary += fmt.Sprintf(", %d warn", warnCount)
		}
		if failCount > 0 {
			summary += fmt.Sprintf(", %d fail", failCount)
		}

		return w.OK(checks,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "link", Command: "ctx ln", Description: "Link packages to agents"},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
