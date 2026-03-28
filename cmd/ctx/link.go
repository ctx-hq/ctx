package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:     "link [agent]",
	Aliases: []string{"ln"},
	Short:   "Link installed packages to an AI agent",
	Long: `Link ctx packages to a specific AI agent's configuration.

Without arguments, lists detected agents. With an agent name,
links all installed packages to that agent.

Supported agents: claude, cursor, windsurf, generic

Examples:
  ctx link                   List detected agents
  ctx link claude            Link all packages to Claude Code
  ctx link cursor            Link all packages to Cursor`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		if len(args) == 0 {
			agents := agent.DetectAll()
			type agentInfo struct {
				Name string `json:"name"`
			}
			result := make([]agentInfo, len(agents))
			for i, a := range agents {
				result[i] = agentInfo{Name: a.Name()}
			}
			return w.OK(result,
				output.WithSummary(fmt.Sprintf("%d agents detected", len(agents))),
				output.WithBreadcrumbs(
					output.Breadcrumb{Action: "link", Command: "ctx ln <agent>", Description: "Link packages to an agent"},
				),
			)
		}

		agentName := args[0]
		a, err := agent.FindByName(agentName)
		if err != nil {
			return output.ErrNotFound("agent", agentName)
		}

		output.Info("Linking packages to %s...", a.Name())

		// Load all installed packages and link them
		inst := installer.NewScanner()

		entries, err := inst.ScanInstalled()
		if err != nil {
			return fmt.Errorf("scan installed: %w", err)
		}

		linked := 0
		for _, entry := range entries {
			manifestPath := filepath.Join(entry.InstallPath, "manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}
			var m manifest.Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}

			switch manifest.PackageType(entry.Type) {
			case manifest.TypeSkill:
				if err := installer.LinkSkillToAgent(entry.InstallPath, m.ShortName(), agentName); err != nil {
					output.Warn("Failed to link skill %s: %v", entry.FullName, err)
					continue
				}
				linked++
			case manifest.TypeMCP:
				if m.MCP != nil {
					cfg := agent.MCPConfig{
						Command: m.MCP.Command,
						Args:    m.MCP.Args,
						URL:     m.MCP.URL,
					}
					if len(m.MCP.Env) > 0 {
						cfg.Env = make(map[string]string)
						for _, e := range m.MCP.Env {
							cfg.Env[e.Name] = e.Default
						}
					}
					if err := a.AddMCP(m.ShortName(), cfg); err != nil {
						output.Warn("Failed to link MCP %s: %v", entry.FullName, err)
						continue
					}
					linked++
				}
			}
		}

		return w.OK(
			map[string]any{"agent": a.Name(), "linked": linked},
			output.WithSummary(fmt.Sprintf("Linked %d packages to %s", linked, a.Name())),
		)
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
