package main

import (
	"fmt"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/getctx/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

var pruneKeep int

var pruneCmd = &cobra.Command{
	Use:   "prune [package]",
	Short: "Remove old package versions",
	Long: `Remove non-current versions of installed packages to free disk space.
The current version is always kept.

Without arguments, prunes all packages. With a package name,
prunes only that package.

Examples:
  ctx prune                     Prune all packages
  ctx prune @hong/review        Prune one package
  ctx prune --keep 2            Keep 2 most recent versions`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		if pruneKeep < 1 {
			return output.ErrUsage("--keep must be at least 1")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		lockPath := config.LockFilePath()
		lf, err := installer.LoadLockFile(lockPath)
		if err != nil {
			return err
		}

		var packages []string
		if len(args) > 0 {
			if !lf.Has(args[0]) {
				return output.ErrNotFound("package", args[0])
			}
			packages = []string{args[0]}
		} else {
			for _, e := range lf.List() {
				packages = append(packages, e.FullName)
			}
		}

		if len(packages) == 0 {
			return w.OK([]any{}, output.WithSummary("no packages to prune"))
		}

		type pruneResult struct {
			FullName string   `json:"full_name"`
			Removed  []string `json:"removed"`
			Freed    int64    `json:"freed_bytes"`
		}

		var results []pruneResult
		var totalFreed int64
		var totalRemoved int

		for _, pkg := range packages {
			removed, freed, err := inst.PruneVersions(pkg, pruneKeep)
			if err != nil {
				output.Warn("Failed to prune %s: %v", pkg, err)
				continue
			}
			if len(removed) > 0 {
				results = append(results, pruneResult{
					FullName: pkg,
					Removed:  removed,
					Freed:    freed,
				})
				totalFreed += freed
				totalRemoved += len(removed)
			}
		}

		summary := "nothing to prune"
		if totalRemoved > 0 {
			summary = fmt.Sprintf("removed %d version(s), freed %s", totalRemoved, formatBytes(totalFreed))
		}

		return w.OK(results, output.WithSummary(summary))
	},
}

func init() {
	pruneCmd.Flags().IntVar(&pruneKeep, "keep", 1, "Number of versions to keep (minimum 1)")
	rootCmd.AddCommand(pruneCmd)
}

// formatBytes formats bytes into human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}
