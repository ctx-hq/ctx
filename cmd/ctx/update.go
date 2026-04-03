package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/ctx-hq/ctx/internal/tui/component"
	"github.com/ctx-hq/ctx/internal/tui/inline"
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
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), getToken())
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

		w.Info("Checking %d package(s) for updates...", len(toCheck))
		resolved, resolveErr := reg.Resolve(cmd.Context(), resolveReq)

		// Phase 2: Determine which packages need updating
		type updateTarget struct {
			entry      installer.InstalledPackage
			newVersion string
		}
		var needsUpdate []updateTarget

		if resolveErr != nil {
			// Batch resolve failed — fall back to sequential
			w.Warn("Batch resolve unavailable, checking individually...")
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

		// Phase 2.5: Let user select which packages to update (TTY + multiple packages + not --yes)
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		if isTTY && !flagYes && !w.IsMachine() && len(needsUpdate) > 1 && len(args) == 0 {
			items := make([]component.MultiSelectorItem, len(needsUpdate))
			for i, t := range needsUpdate {
				items[i] = component.MultiSelectorItem{
					Label:    fmt.Sprintf("%s  %s → %s", t.entry.FullName, t.entry.Version, t.newVersion),
					Selected: true,
				}
			}
			selected, selectErr := inline.SelectFromItems(items, "Select packages to update:")
			if selectErr != nil {
				return selectErr
			}
			if len(selected) == 0 {
				return w.OK([]any{}, output.WithSummary("no packages selected for update"))
			}
			if len(selected) < len(needsUpdate) {
				filtered := make([]updateTarget, 0, len(selected))
				for _, idx := range selected {
					if idx >= 0 && idx < len(needsUpdate) {
						filtered = append(filtered, needsUpdate[idx])
					}
				}
				needsUpdate = filtered
			}
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

		downloadAll := func(_ context.Context, report func(float64)) error {
			sem := make(chan struct{}, maxParallelDownloads)
			var wg sync.WaitGroup
			var completed int64
			var mu sync.Mutex

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
					} else {
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
								SHA256:      res.ArchiveSHA256,
							}
						}
						results[i] = r
					}

					mu.Lock()
					completed++
					report(float64(completed) / float64(len(needsUpdate)))
					mu.Unlock()
				}(idx, target)
			}
			wg.Wait()
			return nil
		}

		if isTTY && !flagYes && !w.IsMachine() {
			if progressErr := inline.RunWithProgress(
				cmd.Context(),
				fmt.Sprintf("Updating %d package(s)", len(needsUpdate)),
				downloadAll,
			); progressErr != nil {
				return progressErr
			}
		} else {
			if dlErr := downloadAll(cmd.Context(), func(float64) {}); dlErr != nil {
				return dlErr
			}
		}

		// Phase 3.5: Sequential post-install linking (avoids concurrent writes to links.json / agent configs)
		for _, r := range results {
			if r.postInstall != nil {
				if err := runPostInstall(cmd, r.postInstall, "", nil); err != nil {
					w.Warn("Post-install link for %s: %v", r.FullName, err)
				}
			}
		}

		// Phase 4: Report results
		updatedCount := 0
		failedCount := 0
		for _, r := range results {
			if r.Updated {
				updatedCount++
				w.Success("%s: %s → %s", r.FullName, r.OldVersion, r.NewVersion)
			} else if r.Error != "" {
				failedCount++
				w.Warn("Failed %s: %s", r.FullName, r.Error)
			}
		}

		summary := fmt.Sprintf("%d updated", updatedCount)
		if failedCount > 0 {
			summary += fmt.Sprintf(", %d failed", failedCount)
		}

		return w.OK(results,
			output.WithSummary(summary),
			output.WithMeta("checked", len(toCheck)),
			output.WithMeta("updated", updatedCount),
			output.WithMeta("failed", failedCount),
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
