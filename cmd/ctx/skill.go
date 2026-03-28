package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/skills"
	"github.com/spf13/cobra"
)

const selfLinkKey = "__self__"

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage ctx's own agent skill",
	Long: `Print or install ctx's embedded SKILL.md for AI agent integration.

Without subcommands, prints the SKILL.md content to stdout
(useful for agent introspection).

Examples:
  ctx skill                Print SKILL.md
  ctx skill install        Install to all detected agents
  ctx skill uninstall      Remove from all agents`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := skills.FS.ReadFile("ctx/SKILL.md")
		if err != nil {
			return fmt.Errorf("read embedded skill: %w", err)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), string(data))
		return err
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install ctx skill to all detected agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// Read embedded SKILL.md
		data, err := skills.FS.ReadFile("ctx/SKILL.md")
		if err != nil {
			return fmt.Errorf("read embedded skill: %w", err)
		}

		// Write to canonical location: ~/.agents/skills/ctx/
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		canonicalDir := filepath.Join(home, ".agents", "skills", "ctx")
		if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
			return err
		}
		skillFile := filepath.Join(canonicalDir, "SKILL.md")
		if err := os.WriteFile(skillFile, data, 0o644); err != nil {
			return err
		}

		// Load link registry
		links, err := installer.LoadLinks()
		if err != nil {
			links = &installer.LinkRegistry{
				Version: 1,
				Links:   make(map[string][]installer.LinkEntry),
			}
		}

		// Register canonical write
		links.Add(selfLinkKey, installer.LinkEntry{
			Agent:  "generic",
			Type:   installer.LinkSymlink,
			Source: canonicalDir,
			Target: canonicalDir,
		})

		// Link to each detected agent
		agents := agent.DetectAll()
		linked := 0
		for _, a := range agents {
			if a.Name() == "generic" {
				linked++ // already written above
				continue
			}
			if err := a.InstallSkill(canonicalDir, "ctx"); err != nil {
				output.Warn("Failed to link to %s: %v", a.Name(), err)
				continue
			}

			// Register link
			links.Add(selfLinkKey, installer.LinkEntry{
				Agent:  a.Name(),
				Type:   installer.LinkSymlink,
				Source: canonicalDir,
				Target: filepath.Join(a.SkillsDir(), "ctx"),
			})
			output.Success("Linked to %s", a.Name())
			linked++
		}

		if err := links.Save(); err != nil {
			return err
		}

		return w.OK(
			map[string]any{"installed": true, "linked_agents": linked},
			output.WithSummary(fmt.Sprintf("ctx skill installed, linked to %d agents", linked)),
		)
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ctx skill from all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// Load and clean links
		links, err := installer.LoadLinks()
		if err != nil {
			return err
		}

		entries := links.Remove(selfLinkKey)
		cleaned := installer.CleanupLinks(entries)
		if err := links.Save(); err != nil {
			return err
		}

		// Also clean canonical dir
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		canonicalDir := filepath.Join(home, ".agents", "skills", "ctx")
		_ = os.RemoveAll(canonicalDir)

		return w.OK(
			map[string]any{"uninstalled": true, "cleaned": cleaned},
			output.WithSummary(fmt.Sprintf("ctx skill removed, cleaned %d links", cleaned)),
		)
	},
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	rootCmd.AddCommand(skillCmd)
}
