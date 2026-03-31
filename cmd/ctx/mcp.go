package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/mcpclient"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/ctx-hq/ctx/internal/secrets"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server management",
	Long: `Manage MCP (Model Context Protocol) servers.

  ctx mcp test @scope/name     Test an installed MCP server
  ctx mcp list                 List installed MCP servers
  ctx mcp env set ...          Manage environment variables`,
}

// --- ctx mcp test ---

var mcpTestTimeout time.Duration

var mcpTestCmd = &cobra.Command{
	Use:   "test [package]",
	Short: "Test an MCP server connection",
	Long: `Test an installed MCP server by connecting, initializing, and
listing tools. Compares returned tools against the manifest declaration.

Examples:
  ctx mcp test @scope/my-mcp     Test a specific package
  ctx mcp test                   Test all installed MCP packages`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		if len(args) == 1 {
			return runMCPTestSingle(cmd.Context(), w, inst, args[0])
		}
		return runMCPTestAll(cmd.Context(), w, inst)
	},
}

func runMCPTestSingle(ctx context.Context, w *output.Writer, inst *installer.Installer, ref string) error {
	m, err := loadInstalledMCPManifest(inst, ref)
	if err != nil {
		return err
	}

	opts := manifestToConnectOpts(m, mcpTestTimeout)
	result, err := runTestWithProgress(ctx, m.ShortName(), opts, m.MCP.Tools)
	if err != nil {
		return err
	}
	return printTestResult(w, m.ShortName(), result)
}

// runTestWithProgress wraps mcpclient.RunTest with a progress indicator.
func runTestWithProgress(ctx context.Context, name string, opts mcpclient.ConnectOptions, tools []string) (*mcpclient.TestResult, error) {
	type testOut struct {
		result *mcpclient.TestResult
		err    error
	}
	ch := make(chan testOut, 1)
	go func() {
		r, e := mcpclient.RunTest(ctx, opts, tools)
		ch <- testOut{r, e}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	dots := 0
	output.PrintDim("  Connecting to %s...", name)

	for {
		select {
		case out := <-ch:
			fmt.Fprint(os.Stderr, "\r\033[K")
			return out.result, out.err
		case <-ticker.C:
			dots++
			fmt.Fprintf(os.Stderr, "\r\033[K")
			output.PrintDim("  Waiting for server to respond... (%ds)", dots*3)
			if dots >= 3 {
				ticker.Stop()
			}
		}
	}
}

func runMCPTestAll(ctx context.Context, w *output.Writer, inst *installer.Installer) error {
	entries, err := inst.ScanInstalled()
	if err != nil {
		return err
	}

	var mcpEntries []installer.InstalledPackage
	for _, e := range entries {
		if e.Type == "mcp" {
			mcpEntries = append(mcpEntries, e)
		}
	}

	if len(mcpEntries) == 0 {
		output.Info("No MCP servers installed.")
		return nil
	}

	for _, e := range mcpEntries {
		m, err := loadManifestFromPath(e.InstallPath)
		if err != nil || m.MCP == nil {
			output.Warn("Skipping %s: cannot load manifest", e.FullName)
			continue
		}

		opts := manifestToConnectOpts(m, mcpTestTimeout)
		result, err := runTestWithProgress(ctx, m.ShortName(), opts, m.MCP.Tools)
		if err != nil {
			output.Warn("Skipping %s: %v", e.FullName, err)
			continue
		}

		_ = printTestResult(w, m.ShortName(), result)
	}

	return nil
}

func printTestResult(w *output.Writer, name string, result *mcpclient.TestResult) error {
	for _, step := range result.Steps {
		var icon string
		switch step.Status {
		case "fail":
			icon = "✗"
		case "skip":
			icon = "○"
		default:
			icon = "✓"
		}
		detail := ""
		if step.Detail != "" {
			detail = " — " + step.Detail
		}
		elapsed := ""
		if step.Elapsed > 0 {
			elapsed = fmt.Sprintf(" (%s)", step.Elapsed.Round(time.Millisecond))
		}
		output.PrintDim("  %s %s%s%s", icon, step.Name, detail, elapsed)
	}

	const stderrPrefix = "  stderr: "
	summary := fmt.Sprintf("%s: %s (%s)", name, result.Status, result.Duration.Round(time.Millisecond))
	if result.Stderr != "" {
		stderr := truncateStderr(result.Stderr, 5, len(stderrPrefix))
		if stderr != "" {
			output.PrintDim("%s%s", stderrPrefix, stderr)
		}
	}

	// Actionable hints on failure
	if result.Status == "fail" {
		for _, step := range result.Steps {
			if step.Status != "fail" {
				continue
			}
			if strings.Contains(step.Detail, "INITIALIZATION_TIMEOUT") {
				fmt.Fprintln(os.Stderr)
				output.Warn("Server did not respond in time. Possible causes:")
				output.PrintDim("    1. First run — npx/docker still downloading (try: --timeout %s)", mcpTestTimeout*2)
				output.PrintDim("    2. Missing auth — server needs credentials (e.g. az login, API key)")
				output.PrintDim("    3. Server error — run manually to see output:")
				output.PrintDim("       %s", formatManualCommand(result))
			} else if strings.Contains(step.Detail, "PROCESS_SPAWN_ERROR") {
				fmt.Fprintln(os.Stderr)
				output.Warn("Could not start the server. Is the command installed?")
				output.PrintDim("    Run manually: %s", formatManualCommand(result))
			}
			break
		}
	}

	return w.OK(result,
		output.WithSummary(summary),
		output.WithMeta("status", result.Status),
	)
}

// --- ctx mcp list ---

var mcpListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		entries, err := inst.ScanInstalled()
		if err != nil {
			return err
		}

		var mcpEntries []installer.InstalledPackage
		for _, e := range entries {
			if e.Type == "mcp" {
				mcpEntries = append(mcpEntries, e)
			}
		}

		return w.OK(mcpEntries,
			output.WithSummary(fmt.Sprintf("%d MCP servers installed", len(mcpEntries))),
			output.WithMeta("total", len(mcpEntries)),
		)
	},
}

// --- ctx mcp env ---

var mcpEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage MCP server environment variables",
	Long: `Store and manage environment variables (API keys, tokens) for MCP servers.
Stored secrets are injected into agent configurations during installation.

  ctx mcp env set @scope/name KEY=value     Set env vars
  ctx mcp env list @scope/name              List stored env vars
  ctx mcp env delete @scope/name KEY        Delete env vars`,
}

var mcpEnvSetCmd = &cobra.Command{
	Use:   "set <package> KEY=value [KEY=value...]",
	Short: "Set environment variables for an MCP server",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		pkg := args[0]

		store, err := secrets.Load()
		if err != nil {
			return fmt.Errorf("load secrets: %w", err)
		}

		var setKeys []string
		for _, kv := range args[1:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return output.ErrUsage(fmt.Sprintf("expected KEY=value, got %q", kv))
			}
			store.Set(pkg, parts[0], parts[1])
			setKeys = append(setKeys, parts[0])
		}

		if err := store.Save(); err != nil {
			return fmt.Errorf("save secrets: %w", err)
		}

		return w.OK(map[string]any{"package": pkg, "keys": setKeys},
			output.WithSummary(fmt.Sprintf("Set %d env var(s) for %s", len(setKeys), pkg)),
		)
	},
}

var mcpEnvShowSecrets bool

var mcpEnvListCmd = &cobra.Command{
	Use:     "list <package>",
	Aliases: []string{"ls"},
	Short:   "List stored environment variables",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		pkg := args[0]

		store, err := secrets.Load()
		if err != nil {
			return fmt.Errorf("load secrets: %w", err)
		}

		m := store.List(pkg)
		if m == nil {
			output.Info("No env vars stored for %s", pkg)
			return w.OK(map[string]any{}, output.WithSummary("No env vars stored"))
		}

		// Mask values unless --show is set
		display := make(map[string]string, len(m))
		for k, v := range m {
			if mcpEnvShowSecrets {
				display[k] = v
			} else {
				display[k] = maskValue(v)
			}
		}

		return w.OK(display,
			output.WithSummary(fmt.Sprintf("%d env var(s) for %s", len(display), pkg)),
		)
	},
}

var mcpEnvDeleteCmd = &cobra.Command{
	Use:     "delete <package> KEY [KEY...]",
	Aliases: []string{"rm"},
	Short:   "Delete environment variables",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		pkg := args[0]

		store, err := secrets.Load()
		if err != nil {
			return fmt.Errorf("load secrets: %w", err)
		}

		for _, key := range args[1:] {
			store.Delete(pkg, key)
		}

		if err := store.Save(); err != nil {
			return fmt.Errorf("save secrets: %w", err)
		}

		return w.OK(map[string]any{"package": pkg, "deleted": args[1:]},
			output.WithSummary(fmt.Sprintf("Deleted %d env var(s) from %s", len(args)-1, pkg)),
		)
	},
}

// --- helpers ---

func loadInstalledMCPManifest(inst *installer.Installer, ref string) (*manifest.Manifest, error) {
	entries, err := inst.ScanInstalled()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.Type != "mcp" {
			continue
		}
		if e.FullName == ref || manifest.FormatFullName("", e.FullName) == ref || strings.HasSuffix(e.FullName, "/"+ref) {
			return loadManifestFromPath(e.InstallPath)
		}
	}
	return nil, fmt.Errorf("MCP server %q not found; is it installed?", ref)
}

func loadManifestFromPath(installPath string) (*manifest.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(installPath, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func manifestToConnectOpts(m *manifest.Manifest, timeout time.Duration) mcpclient.ConnectOptions {
	opts := mcpclient.ConnectOptions{
		Transport: m.MCP.Transport,
		Command:   m.MCP.Command,
		Args:      m.MCP.Args,
		URL:       m.MCP.URL,
		Timeout:   timeout,
	}

	// Build env from manifest defaults + secrets store
	env := make(map[string]string)
	for _, e := range m.MCP.Env {
		if e.Default != "" {
			env[e.Name] = e.Default
		}
	}
	if store, err := secrets.Load(); err != nil {
		output.Warn("load secrets: %v", err)
	} else {
		for k, v := range store.List(m.Name) {
			env[k] = v
		}
	}
	if len(env) > 0 {
		opts.Env = env
	}

	return opts
}

// stderrNoisePatterns lists substrings used to filter noisy stderr lines
// (e.g. Docker pull progress) from MCP server test output.
var stderrNoisePatterns = []string{
	": Pulling fs layer",
	": Waiting",
	": Download complete",
	": Pull complete",
	": Verifying Checksum",
}

// truncateStderr returns the last maxLines non-empty lines of stderr output.
// Lines are indented by prefixLen spaces for alignment. Returns empty string
// if stderr is all whitespace.
func truncateStderr(stderr string, maxLines, prefixLen int) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	// Filter out empty lines and docker pull progress noise
	var meaningful []string
outer:
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, pattern := range stderrNoisePatterns {
			if strings.Contains(line, pattern) {
				continue outer
			}
		}
		meaningful = append(meaningful, line)
	}
	if len(meaningful) == 0 {
		return ""
	}
	if len(meaningful) > maxLines {
		meaningful = meaningful[len(meaningful)-maxLines:]
	}
	return strings.Join(meaningful, "\n"+strings.Repeat(" ", prefixLen))
}

// formatManualCommand returns a copy-pasteable command from the test result.
func formatManualCommand(result *mcpclient.TestResult) string {
	if result.Command != "" {
		return result.Command
	}
	return "(command not available)"
}

func maskValue(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(v)-4) + v[len(v)-4:]
}

func init() {
	mcpTestCmd.Flags().DurationVar(&mcpTestTimeout, "timeout", 120*time.Second, "Test timeout")
	mcpEnvListCmd.Flags().BoolVar(&mcpEnvShowSecrets, "show", false, "Show actual values (default: masked)")

	mcpEnvCmd.AddCommand(mcpEnvSetCmd, mcpEnvListCmd, mcpEnvDeleteCmd)
	mcpCmd.AddCommand(mcpTestCmd, mcpListCmd, mcpEnvCmd)
	rootCmd.AddCommand(mcpCmd)
}
