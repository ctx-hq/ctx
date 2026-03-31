package app

import (
	"context"
	"os"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/registry"
)

// Service defines the operations the TUI needs from the backend.
type Service interface {
	ScanInstalled() ([]installer.InstalledPackage, error)
	Search(ctx context.Context, query, pkgType string, limit, offset int) (*registry.SearchResult, error)
	GetPackage(ctx context.Context, fullName string) (*registry.PackageDetail, error)
	Install(ctx context.Context, ref string) (*installer.InstallResult, error)
	Remove(ctx context.Context, fullName string) error
	DetectAgents() []agentInfo
	RunDoctorChecks() *doctor.Result
}

// RealService implements Service by delegating to real packages.
type RealService struct {
	Installer *installer.Installer
	Registry  *registry.Client
	Version   string
	Token     string
}

// ScanInstalled returns all installed packages.
func (s *RealService) ScanInstalled() ([]installer.InstalledPackage, error) {
	return s.Installer.ScanInstalled()
}

// Search searches the registry for packages.
func (s *RealService) Search(ctx context.Context, query, pkgType string, limit, offset int) (*registry.SearchResult, error) {
	return s.Registry.SearchWithOffset(ctx, query, pkgType, "", limit, offset)
}

// GetPackage fetches full package details from the registry.
func (s *RealService) GetPackage(ctx context.Context, fullName string) (*registry.PackageDetail, error) {
	return s.Registry.GetPackage(ctx, fullName)
}

// Install installs a package by reference.
func (s *RealService) Install(ctx context.Context, ref string) (*installer.InstallResult, error) {
	return s.Installer.Install(ctx, ref)
}

// Remove uninstalls a package.
func (s *RealService) Remove(ctx context.Context, fullName string) error {
	return s.Installer.Remove(ctx, fullName)
}

// DetectAgents finds all agents and counts their installed skills.
func (s *RealService) DetectAgents() []agentInfo {
	agents := agent.DetectAll()
	var result []agentInfo
	for _, a := range agents {
		skillCount := 0
		dir := a.SkillsDir()
		if entries, err := os.ReadDir(dir); err == nil {
			skillCount = len(entries)
		}
		result = append(result, agentInfo{
			Name:       a.Name(),
			SkillsDir:  dir,
			SkillCount: skillCount,
		})
	}
	return result
}

// RunDoctorChecks runs all diagnostic checks.
func (s *RealService) RunDoctorChecks() *doctor.Result {
	return doctor.RunChecks(s.Version, s.Token)
}
