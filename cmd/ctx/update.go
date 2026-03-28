package main

import (
	"fmt"
	"sync"

	"github.com/getctx/ctx/internal/config"
	"github.com/getctx/ctx/internal/installer"
	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/registry"
	"github.com/getctx/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

const maxParallelDownloads = 4

var updateCmd = &cobra.Command{
	Use:     "update [package]",
	Aliases: []string{"up"},
	Short:   "Update installed packages",
	Long: `Update one or all installed packages to their latest versions.

Without arguments, updates all packages. With a package name,
updates only that package.

Uses batch version resolution (1 HTTP request) and parallel downloads
for optimal performance.

Examples:
  ctx update                    Update all packages
  ctx update @hong/my-skill     Update specific package`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), cfg.Token)
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		var toCheck []installer.InstalledPackage
		if len(args) > 0 {
			pkg, err := inst.GetInstalled(args[0])
			if err != nil {
				return output.ErrNotFound("package", args[0])
			}
			toCheck = []installer.InstalledPackage{*pkg}
		} else {
			all, err := inst.ScanInstalled()
			if err != nil {
				return err
			}
			toCheck = all
		}

		if len(toCheck) == 0 {
			return w.OK([]any{}, output.WithSummary("no packages to update"))
		}

		// Phase 1: Batch resolve — 1 HTTP request for all packages
		// Use "*" (latest) since the API resolves to latest non-yanked version
		// Client-side comparison determines if update is needed
		resolveReq := &registry.ResolveRequest{
			Packages: make(map[string]string, len(toCheck)),
		}
		for _, e := range toCheck {
			resolveReq.Packages[e.FullName] = "*"
		}

		output.Info("Checking %d package(s) for updates...", len(toCheck))
		resolved, resolveErr := reg.Resolve(cmd.Context(), resolveReq)

		// Phase 2: Determine which packages need updating
		type updateTarget struct {
			entry      installer.InstalledPackage
			newVersion string
		}
		var needsUpdate []updateTarget

		if resolveErr != nil {
			// Batch resolve failed — fall back to sequential
			output.Warn("Batch resolve unavailable, checking individually...")
			for _, e := range toCheck {
				pkg, err := reg.GetPackage(cmd.Context(), e.FullName)
				if err != nil {
					continue
				}
				if pkg.Version != "" && pkg.Version != e.Version {
					needsUpdate = append(needsUpdate, updateTarget{entry: e, newVersion: pkg.Version})
				}
			}
		} else {
			for _, e := range toCheck {
				if r, ok := resolved.Resolved[e.FullName]; ok && r.Version != e.Version {
					needsUpdate = append(needsUpdate, updateTarget{entry: e, newVersion: r.Version})
				}
			}
		}

		if len(needsUpdate) == 0 {
			return w.OK([]any{},
				output.WithSummary("all packages are up to date"),
			)
		}

		// Phase 3: Parallel download with bounded concurrency
		type updateResult struct {
			FullName   string `json:"full_name"`
			OldVersion string `json:"old_version"`
			NewVersion string `json:"new_version"`
			Updated    bool   `json:"updated"`
			Error      string `json:"error,omitempty"`
			// unexported fields for post-install (not serialized)
			postInstall *installer.InstallResult
		}

		results := make([]updateResult, len(needsUpdate))
		sem := make(chan struct{}, maxParallelDownloads)
		var wg sync.WaitGroup

		for idx, target := range needsUpdate {
			wg.Add(1)
			go func(i int, t updateTarget) {
				defer wg.Done()
				sem <- struct{}{}        // acquire
				defer func() { <-sem }() // release

				res, m, err := inst.InstallFiles(cmd.Context(), t.entry.FullName+"@"+t.newVersion)
				if err != nil {
					results[i] = updateResult{
						FullName:   t.entry.FullName,
						OldVersion: t.entry.Version,
						NewVersion: t.entry.Version,
						Updated:    false,
						Error:      err.Error(),
					}
					return
				}

				r := updateResult{
					FullName:   t.entry.FullName,
					OldVersion: t.entry.Version,
					NewVersion: res.Version,
					Updated:    res.Version != t.entry.Version,
				}
				if m != nil {
					r.postInstall = &installer.InstallResult{
						FullName:    res.FullName,
						Version:     res.Version,
						Type:        string(m.Type),
						InstallPath: inst.CurrentLink(res.FullName),
						Source:      res.Source,
					}
				}
				results[i] = r
			}(idx, target)
		}
		wg.Wait()

		// Phase 3.5: Sequential post-install linking (avoids concurrent writes to links.json / agent configs)
		for _, r := range results {
			if r.postInstall != nil {
				if err := runPostInstall(cmd, r.postInstall, ""); err != nil {
					output.Warn("Post-install link for %s: %v", r.FullName, err)
				}
			}
		}

		// Phase 4: Report results
		updatedCount := 0
		failedCount := 0
		for _, r := range results {
			if r.Updated {
				updatedCount++
				output.Success("%s: %s → %s", r.FullName, r.OldVersion, r.NewVersion)
			} else if r.Error != "" {
				failedCount++
				output.Warn("Failed %s: %s", r.FullName, r.Error)
			}
		}

		summary := fmt.Sprintf("%d updated", updatedCount)
		if failedCount > 0 {
			summary += fmt.Sprintf(", %d failed", failedCount)
		}

		return w.OK(results,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx ls", Description: "List installed packages"},
				output.Breadcrumb{Action: "outdated", Command: "ctx od", Description: "Check for updates"},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
