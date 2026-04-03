package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Cross-device package sync",
	Long: `Sync your installed packages across devices.

Examples:
  ctx sync export     Export installed state to local file
  ctx sync push       Upload sync profile to registry
  ctx sync pull       Restore packages from sync profile
  ctx sync status     View sync status and last sync time`,
}

var syncExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export installed packages to a local sync profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		profile, err := buildSyncProfile()
		if err != nil {
			return err
		}

		// Write to ~/.ctx/sync-profile.json
		profilePath := filepath.Join(config.DataDir(), "sync-profile.json")
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(profilePath), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(profilePath, data, 0o600); err != nil {
			return err
		}

		syncable := 0
		for _, p := range profile.Packages {
			if p.Syncable {
				syncable++
			}
		}
		unsyncable := len(profile.Packages) - syncable

		result := map[string]interface{}{
			"file":       profilePath,
			"total":      len(profile.Packages),
			"syncable":   syncable,
			"unsyncable": unsyncable,
		}

		opts := []output.ResponseOption{
			output.WithSummary(fmt.Sprintf("Exported %d packages (%d syncable, %d unsyncable)", len(profile.Packages), syncable, unsyncable)),
		}
		if unsyncable > 0 {
			opts = append(opts, output.WithNotice(fmt.Sprintf("%d packages have no remote source. Run 'ctx push' in their directories to make them syncable.", unsyncable)))
		}

		opts = append(opts, output.WithBreadcrumbs(
			output.Breadcrumb{Action: "push", Command: "ctx sync push", Description: "Upload profile to registry"},
		))

		return w.OK(result, opts...)
	},
}

var syncPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload sync profile to registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in — run 'ctx login' first")
		}

		profile, err := buildSyncProfile()
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		if err := reg.PushSyncProfile(cmd.Context(), profile); err != nil {
			return err
		}

		syncable := 0
		for _, p := range profile.Packages {
			if p.Syncable {
				syncable++
			}
		}

		return w.OK(
			map[string]interface{}{"uploaded": true, "packages": len(profile.Packages)},
			output.WithSummary(fmt.Sprintf("Profile uploaded (%d packages, %d syncable)", len(profile.Packages), syncable)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "pull", Command: "ctx sync pull", Description: "Restore on another device"},
			),
		)
	},
}

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Restore packages from sync profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in — run 'ctx login' first")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		resp, err := reg.GetSyncProfile(cmd.Context())
		if err != nil {
			return err
		}

		// Record pull event
		hostname, _ := os.Hostname()
		_ = reg.RecordSyncPull(cmd.Context(), hostname)

		restored := 0
		skipped := 0
		inst := installer.New(reg, resolver.New(reg))
		for _, pkg := range resp.Profile.Packages {
			if !pkg.Syncable {
				skipped++
				w.Warn("Skipped %s (no remote source)", pkg.Name)
				continue
			}

			w.Info("Restoring %s...", pkg.Name)
			var ref string
			if pkg.SourceURL != "" {
				// Use explicit source URL (e.g. github:owner/repo@ref)
				ref = pkg.SourceURL
			} else {
				ref = pkg.Name
				if pkg.Constraint != "" {
					ref = pkg.Name + "@" + pkg.Constraint
				}
			}
			if _, installErr := inst.Install(cmd.Context(), ref); installErr != nil {
				w.Warn("Failed to restore %s: %v", pkg.Name, installErr)
				continue
			}
			restored++
		}

		return w.OK(
			map[string]interface{}{"restored": restored, "skipped": skipped},
			output.WithSummary(fmt.Sprintf("Restored %d/%d packages", restored, len(resp.Profile.Packages))),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx ls", Description: "List installed packages"},
				output.Breadcrumb{Action: "doctor", Command: "ctx doctor", Description: "Verify installation health"},
			),
		)
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "View sync status and last sync time",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in — run 'ctx login' first")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), token)
		resp, err := reg.GetSyncProfile(cmd.Context())
		if err != nil {
			if registry.IsNotFound(err) {
				return w.OK(
					map[string]string{"status": "no profile"},
					output.WithSummary("No sync profile found. Run 'ctx sync push' to create one."),
				)
			}
			return err
		}

		return w.OK(resp.Meta,
			output.WithSummary(fmt.Sprintf(
				"Synced %d packages (%d syncable)",
				resp.Meta.PackageCount, resp.Meta.SyncableCount,
			)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "pull", Command: "ctx sync pull", Description: "Restore on this device"},
				output.Breadcrumb{Action: "push", Command: "ctx sync push", Description: "Upload latest state"},
			),
		)
	},
}

func init() {
	syncCmd.AddCommand(syncExportCmd)
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)
	syncCmd.AddCommand(syncStatusCmd)
}

// buildSyncProfile scans installed packages and builds a sync profile.
func buildSyncProfile() (*registry.SyncProfile, error) {
	scanner := installer.NewScanner()
	installed, err := scanner.ScanInstalled()
	if err != nil {
		return nil, fmt.Errorf("scan installed packages: %w", err)
	}

	hostname, _ := os.Hostname()

	profile := &registry.SyncProfile{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Device:     hostname,
	}

	for _, pkg := range installed {
		entry := registry.SyncPackageEntry{
			Name:    pkg.FullName,
			Version: pkg.Version,
			Agents:  []string{},
		}

		// Pin the exact installed version as constraint for reproducible restore
		if pkg.Version != "" {
			entry.Constraint = pkg.Version
		}

		// Read manifest for visibility and source info
		manifestPath := filepath.Join(pkg.InstallPath, "manifest.json")
		if data, readErr := os.ReadFile(manifestPath); readErr == nil {
			var m manifest.Manifest
			if json.Unmarshal(data, &m) == nil {
				entry.Visibility = m.Visibility

				// Extract source URL from manifest for GitHub-sourced packages
				if m.Source != nil && m.Source.GitHub != "" {
					entry.SourceURL = "github:" + m.Source.GitHub
					if m.Source.Ref != "" {
						entry.SourceURL += "@" + m.Source.Ref
					}
				}
			}
		}

		// Determine source and syncability
		if entry.SourceURL != "" {
			// Package has an explicit source URL (e.g. GitHub direct install)
			entry.Source = "github"
			entry.Syncable = true
		} else if pkg.FullName != "" && pkg.FullName[0] == '@' {
			entry.Source = "registry"
			entry.Syncable = true
		} else {
			entry.Source = "local"
			entry.Syncable = false
		}

		profile.Packages = append(profile.Packages, entry)
	}

	return profile, nil
}
