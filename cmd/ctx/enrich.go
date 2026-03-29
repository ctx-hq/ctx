package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var enrichResetFlag bool

var enrichCmd = &cobra.Command{
	Use:   "enrich <package>",
	Short: "View or manage SKILL.md enrichment",
	Long: `View or reset enrichment of a package's SKILL.md.

If a package's SKILL.md has been enriched (e.g. via 'ctx enrich --apply'),
this command lets you inspect the enrichment or restore the original.

Examples:
  ctx enrich @scope/name           View enrichment details
  ctx enrich @scope/name --reset   Restore original SKILL.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		scanner := installer.NewScanner()
		installed, err := scanner.ScanInstalled()
		if err != nil {
			return err
		}

		// Find the package
		var found *installer.InstalledPackage
		for i := range installed {
			if installed[i].FullName == args[0] {
				found = &installed[i]
				break
			}
		}

		if found == nil {
			return output.ErrUsageHint(
				fmt.Sprintf("package %s not found", args[0]),
				"Run 'ctx list' to see installed packages",
			)
		}

		originalPath := filepath.Join(found.InstallPath, "SKILL.md.original")
		skillPath := filepath.Join(found.InstallPath, "SKILL.md")
		manifestPath := filepath.Join(found.InstallPath, "manifest.json")

		if enrichResetFlag {
			// Reset: restore original
			if _, err := os.Stat(originalPath); os.IsNotExist(err) {
				return output.ErrUsage("no enrichment to reset (no .original backup found)")
			}
			original, err := os.ReadFile(originalPath)
			if err != nil {
				return fmt.Errorf("read original: %w", err)
			}
			if err := os.WriteFile(skillPath, original, 0o644); err != nil {
				return fmt.Errorf("write SKILL.md: %w", err)
			}
			if removeErr := os.Remove(originalPath); removeErr != nil {
				output.Warn("Could not remove backup: %v", removeErr)
			}
			output.Info("Enrichment removed, original SKILL.md restored")
			return w.OK(map[string]string{"action": "reset", "package": args[0]},
				output.WithSummary("Enrichment removed for "+args[0]),
			)
		}

		// View enrichment details
		hasOriginal := false
		if _, err := os.Stat(originalPath); err == nil {
			hasOriginal = true
		}

		result := map[string]interface{}{
			"package":        args[0],
			"has_enrichment": hasOriginal,
			"install_path":   found.InstallPath,
		}

		// Read manifest for enrichment metadata
		if data, err := os.ReadFile(manifestPath); err == nil {
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil {
				if enrichment, ok := m["_enrichment"]; ok {
					result["enrichment"] = enrichment
				}
			}
		}

		summary := "No enrichment applied"
		if hasOriginal {
			summary = "Enrichment active (original preserved at SKILL.md.original)"
		}

		return w.OK(result, output.WithSummary(summary))
	},
}

func init() {
	enrichCmd.Flags().BoolVar(&enrichResetFlag, "reset", false, "Restore original SKILL.md")
}
