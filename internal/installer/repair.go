package installer

import (
	"context"
	"os"

	"github.com/ctx-hq/ctx/internal/adapter"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
)

// RepairReport summarizes what was checked and repaired.
type RepairReport struct {
	CLIStatus string              // "ok", "repaired", "failed", "n/a"
	Skills    []SkillRepairStatus // per-agent status
	Repaired  int
	AlreadyOK int
}

// SkillRepairStatus tracks repair status for a single agent.
type SkillRepairStatus struct {
	Agent  string
	Status string // "ok", "repaired", "failed"
}

// Repair verifies and fixes an already-installed package's components.
func Repair(ctx context.Context, m *manifest.Manifest, pkgDir, installPath, fullName, caller string) (*RepairReport, error) {
	state, _ := installstate.Load(pkgDir)
	if state == nil {
		return nil, nil // no state to repair from
	}

	report := &RepairReport{CLIStatus: "n/a"}

	// Check CLI binary
	if state.CLI != nil {
		if err := adapter.Verify(state.CLI.Binary, ""); err != nil {
			// Binary is missing — try to reinstall via same adapter
			output.Info("CLI binary %s not found, reinstalling via %s...", state.CLI.Binary, state.CLI.Adapter)
			a, aErr := adapter.FindByName(state.CLI.Adapter)
			if aErr == nil {
				if installErr := a.Install(ctx, state.CLI.AdapterPkg); installErr == nil {
					if verifyErr := adapter.Verify(state.CLI.Binary, ""); verifyErr == nil {
						state.CLI.Status = "ok"
						state.CLI.Verified = true
						report.CLIStatus = "repaired"
						report.Repaired++
					} else {
						state.CLI.Status = "failed"
						report.CLIStatus = "failed"
					}
				} else {
					state.CLI.Status = "failed"
					report.CLIStatus = "failed"
				}
			} else {
				report.CLIStatus = "failed"
			}
		} else {
			report.CLIStatus = "ok"
			report.AlreadyOK++
		}
	}

	// Check skill symlinks
	for i, s := range state.Skills {
		if _, err := os.Stat(s.SymlinkPath); err != nil {
			// Symlink is broken — re-link
			output.Info("Skill link for %s broken, re-linking...", s.Agent)
			if m != nil {
				_, _ = LinkSkillToAgents(installPath, m.ShortName(), fullName, caller)
			}
			// Re-check
			if _, err := os.Stat(s.SymlinkPath); err == nil {
				state.Skills[i].Status = "ok"
				report.Skills = append(report.Skills, SkillRepairStatus{Agent: s.Agent, Status: "repaired"})
				report.Repaired++
			} else {
				state.Skills[i].Status = "broken"
				report.Skills = append(report.Skills, SkillRepairStatus{Agent: s.Agent, Status: "failed"})
			}
		} else {
			report.Skills = append(report.Skills, SkillRepairStatus{Agent: s.Agent, Status: "ok"})
			report.AlreadyOK++
		}
	}

	// Save updated state
	_ = state.Save(pkgDir)

	return report, nil
}
