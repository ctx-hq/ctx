package main

import (
	"fmt"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/resolver"
	"github.com/ctx-hq/ctx/internal/securityscan"
	"github.com/spf13/cobra"
)

// AuditEntry is the result for a single package in the audit.
type AuditEntry struct {
	FullName        string                 `json:"full_name"`
	Version         string                 `json:"version"`
	Source          string                 `json:"source,omitempty"`
	HasIntegrity    bool                   `json:"has_integrity"`     // whether archive_sha256 was recorded
	Unavailable     bool                   `json:"unavailable"`       // version no longer available upstream
	ScanFindings    int                    `json:"scan_findings"`
	ScanPassed      bool                   `json:"scan_passed"`
	Status          string                 `json:"status"` // "ok", "unavailable", "scan_failed"
	SecurityDetails []securityscan.Finding `json:"security_details,omitempty"`
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Verify integrity and security of installed packages",
	Long: `Audit checks all installed packages for:
  - Integrity: verifies archive SHA256 was recorded at install time
  - Availability: checks if installed version is still available upstream
  - Security: scans package files for dangerous patterns

Examples:
  ctx audit              Audit all installed packages
  ctx audit --json       Machine-readable output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		reg := registry.New(cfg.RegistryURL(), getToken())
		res := resolver.New(reg)
		inst := installer.New(reg, res)

		installed, err := inst.ScanInstalled()
		if err != nil {
			return fmt.Errorf("scan installed: %w", err)
		}
		if len(installed) == 0 {
			return w.OK([]AuditEntry{}, output.WithSummary("No packages installed"))
		}

		output.Info("Auditing %d installed package(s)...", len(installed))

		// Batch resolve to check unavailable versions (best-effort, skip on error)
		unavailableSet := map[string]bool{}
		if !flagOffline {
			resolveReq := &registry.ResolveRequest{
				Packages: make(map[string]string, len(installed)),
			}
			for _, pkg := range installed {
				resolveReq.Packages[pkg.FullName] = pkg.Version
			}
			if resp, resolveErr := reg.Resolve(cmd.Context(), resolveReq); resolveErr == nil {
				for name, resolved := range resp.Resolved {
					if resolved.Version == "" {
						unavailableSet[name] = true
					}
				}
			}
		}

		var entries []AuditEntry
		okCount, unavailableCount, scanFail := 0, 0, 0

		for _, pkg := range installed {
			entry := AuditEntry{
				FullName:   pkg.FullName,
				Version:    pkg.Version,
				ScanPassed: true,
				Status:     "ok",
			}

			// Read state.json for integrity metadata (state.json is saved at package root, not version subdir)
			pkgDir := inst.PackageDir(pkg.FullName)
			if state, stateErr := installstate.Load(pkgDir); stateErr == nil && state != nil {
				entry.Source = state.Source
				entry.HasIntegrity = state.ArchiveSHA256 != ""
			}

			// Security scan — resolve symlink so WalkDir traverses the real directory
			currentDir := inst.CurrentLink(pkg.FullName)
			if resolved, resolveErr := filepath.EvalSymlinks(currentDir); resolveErr == nil {
				currentDir = resolved
			}
			if scanResult, scanErr := securityscan.Scan(currentDir); scanErr == nil {
				entry.ScanFindings = len(scanResult.Findings)
				entry.ScanPassed = scanResult.Passed()
				if !scanResult.Passed() {
					entry.SecurityDetails = scanResult.Findings
				}
			}

			// Determine final status: unavailable takes precedence over scan_failed
			switch {
			case unavailableSet[pkg.FullName]:
				entry.Unavailable = true
				entry.Status = "unavailable"
				unavailableCount++
			case !entry.ScanPassed:
				entry.Status = "scan_failed"
				scanFail++
			default:
				okCount++
			}

			// Print human-readable line
			if !w.IsMachine() {
				switch entry.Status {
				case "ok":
					integrityStr := ""
					if entry.HasIntegrity {
						integrityStr = ", integrity recorded"
					}
					output.Success("  %s@%s — ok%s", entry.FullName, entry.Version, integrityStr)
				case "unavailable":
					output.Warn("  %s@%s — unavailable upstream", entry.FullName, entry.Version)
				case "scan_failed":
					output.Warn("  %s@%s — %d security finding(s)", entry.FullName, entry.Version, entry.ScanFindings)
				}
			}

			entries = append(entries, entry)
		}

		summary := fmt.Sprintf("%d ok", okCount)
		if unavailableCount > 0 {
			summary += fmt.Sprintf(", %d unavailable", unavailableCount)
		}
		if scanFail > 0 {
			summary += fmt.Sprintf(", %d scan failed", scanFail)
		}

		return w.OK(entries,
			output.WithSummary(summary),
			output.WithMeta("total", len(installed)),
			output.WithMeta("ok", okCount),
			output.WithMeta("unavailable", unavailableCount),
			output.WithMeta("scan_failed", scanFail),
		)
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
