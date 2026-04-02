package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
)

const hookTimeout = 5 * time.Minute

// RunPostInstallHooks executes the post_install hooks declared in mcp.hooks.
// Returns a list of completed hook descriptions for state tracking.
// Required hooks (optional=false) cause an error on failure; optional hooks warn and continue.
// If confirm is non-nil, hooks are shown to the user and confirm is called;
// returning false skips all hooks. Pass nil to run without confirmation (e.g. --yes).
func RunPostInstallHooks(ctx context.Context, m *manifest.Manifest, confirm func() (bool, error)) ([]string, error) {
	if m.MCP == nil || m.MCP.Hooks == nil || len(m.MCP.Hooks.PostInstall) == 0 {
		return nil, nil
	}

	// Display all commands for user review
	output.Info("Post-install hooks to run:")
	for _, step := range m.MCP.Hooks.PostInstall {
		cmd := step.Command
		if len(step.Args) > 0 {
			cmd += " " + strings.Join(step.Args, " ")
		}
		output.PrintDim("  %s", cmd)
	}

	// Ask for confirmation if a confirm callback is provided
	if confirm != nil {
		ok, err := confirm()
		if err != nil {
			return nil, err
		}
		if !ok {
			output.Warn("Post-install hooks skipped by user")
			return nil, nil
		}
	}

	var completed []string
	for _, step := range m.MCP.Hooks.PostInstall {
		desc := step.Description
		if desc == "" {
			desc = step.Command
		}
		output.Info("Running: %s", desc)

		hookCtx, cancel := context.WithTimeout(ctx, hookTimeout)
		cmd := exec.CommandContext(hookCtx, step.Command, step.Args...)
		cmd.Stdout = nil // suppress stdout
		cmd.Stderr = nil // suppress stderr

		err := cmd.Run()
		cancel()

		if err != nil {
			if step.Optional {
				output.Warn("Optional hook failed: %s: %v", desc, err)
				continue
			}
			return completed, fmt.Errorf("post-install hook failed: %s: %w", desc, err)
		}
		completed = append(completed, desc)
		output.PrintDim("  Hook completed: %s", desc)
	}

	return completed, nil
}
