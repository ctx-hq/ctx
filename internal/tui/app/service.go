package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/agent"
	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/registry"
)

// FileInfo holds metadata about a file in a package directory.
type FileInfo struct {
	Name  string
	Size  int64
	IsDir bool
}

// AgentSkillEntry represents a skill entry in an agent's skills directory.
type AgentSkillEntry struct {
	Name       string
	IsSymlink  bool
	LinkTarget string // resolved symlink target, e.g. "@biao29/gc"
}

// AgentMCPEntry represents an MCP server configured for an agent.
type AgentMCPEntry struct {
	Name    string
	Command string
}

// Service defines the operations the TUI needs from the backend.
type Service interface {
	ScanInstalled() ([]installer.InstalledPackage, error)
	Search(ctx context.Context, query, pkgType string, limit, offset int) (*registry.SearchResult, error)
	GetPackage(ctx context.Context, fullName string) (*registry.PackageDetail, error)
	Install(ctx context.Context, ref string) (*installer.InstallResult, error)
	Remove(ctx context.Context, fullName string) error
	DetectAgents() []agentInfo
	RunDoctorChecks() *doctor.Result
	GetPackageState(fullName string) *installstate.PackageState
	ListPackageFiles(fullName string) ([]FileInfo, error)
	ReadPackageFile(fullName, fileName string) (string, error)
	GetSkillContent(fullName string) string
	GetAgentDetail(agentName string) ([]AgentSkillEntry, []AgentMCPEntry)
	ListDirFiles(dir string) ([]FileInfo, error)
	ReadDirFile(dir, name string) (string, error)
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

// GetPackageState loads the install state for a package.
func (s *RealService) GetPackageState(fullName string) *installstate.PackageState {
	pkgDir := s.Installer.PackageDir(fullName)
	if pkgDir == "" {
		pkgDir = config.DataDir() + "/packages/" + fullName
	}
	state, _ := installstate.Load(pkgDir)
	return state
}

// ListPackageFiles returns files in the package's current/ directory.
func (s *RealService) ListPackageFiles(fullName string) ([]FileInfo, error) {
	currentDir := s.Installer.CurrentLink(fullName)
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}
	var files []FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:  e.Name(),
			Size:  info.Size(),
			IsDir: e.IsDir(),
		})
	}
	return files, nil
}

// ReadPackageFile reads the content of a file in the package's current/ directory.
func (s *RealService) ReadPackageFile(fullName, fileName string) (string, error) {
	currentDir := s.Installer.CurrentLink(fullName)
	filePath := filepath.Join(currentDir, fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetSkillContent reads SKILL.md from a package directory, returning empty string if not found.
func (s *RealService) GetSkillContent(fullName string) string {
	currentDir := s.Installer.CurrentLink(fullName)
	data, err := os.ReadFile(filepath.Join(currentDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// GetAgentDetail returns the skills and MCP servers for a given agent.
func (s *RealService) GetAgentDetail(agentName string) ([]AgentSkillEntry, []AgentMCPEntry) {
	agents := agent.DetectAll()
	var matched agent.Agent
	for _, a := range agents {
		if a.Name() == agentName {
			matched = a
			break
		}
	}
	if matched == nil {
		return nil, nil
	}

	// Read skills directory.
	skillsDir := matched.SkillsDir()
	var skills []AgentSkillEntry
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			entry := AgentSkillEntry{Name: e.Name()}
			fullPath := filepath.Join(skillsDir, e.Name())
			if target, err := os.Readlink(fullPath); err == nil {
				entry.IsSymlink = true
				entry.LinkTarget = target
			}
			skills = append(skills, entry)
		}
	}

	// Read MCP config from parent of skillsDir.
	configDir := filepath.Dir(skillsDir)
	mcpPath := filepath.Join(configDir, "mcp.json")
	var mcpServers []AgentMCPEntry
	if data, err := os.ReadFile(mcpPath); err == nil {
		var parsed map[string]json.RawMessage
		if json.Unmarshal(data, &parsed) == nil {
			if serversRaw, ok := parsed["mcpServers"]; ok {
				var servers map[string]json.RawMessage
				if json.Unmarshal(serversRaw, &servers) == nil {
					for name, raw := range servers {
						var cfg struct {
							Command string   `json:"command"`
							Args    []string `json:"args"`
						}
						if json.Unmarshal(raw, &cfg) == nil {
							cmd := cfg.Command
							if len(cfg.Args) > 0 {
								cmd += " " + strings.Join(cfg.Args, " ")
							}
							mcpServers = append(mcpServers, AgentMCPEntry{
								Name:    name,
								Command: cmd,
							})
						}
					}
				}
			}
		}
	}

	return skills, mcpServers
}

// ListDirFiles returns files in an arbitrary directory.
// Symlinks pointing to directories are treated as directories.
func (s *RealService) ListDirFiles(dir string) ([]FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []FileInfo
	for _, e := range entries {
		if e.Name() == ".DS_Store" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		isDir := e.IsDir()
		// Resolve symlinks: check if target is a directory.
		if info.Mode()&os.ModeSymlink != 0 {
			fullPath := filepath.Join(dir, e.Name())
			if resolved, err := os.Stat(fullPath); err == nil {
				isDir = resolved.IsDir()
				if !isDir {
					// Use resolved file size for symlink-to-file.
					info = resolved
				}
			}
		}
		files = append(files, FileInfo{
			Name:  e.Name(),
			Size:  info.Size(),
			IsDir: isDir,
		})
	}
	return files, nil
}

// ReadDirFile reads the content of a file in a directory.
// If the path is a directory (e.g. symlink to skill dir), tries to read SKILL.md inside.
func (s *RealService) ReadDirFile(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	// If it's a directory, try reading SKILL.md inside.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		skillPath := filepath.Join(path, "SKILL.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			return string(data), nil
		}
		// No SKILL.md — return a listing of the directory contents.
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}
		var listing strings.Builder
		listing.WriteString("# " + name + "/\n\n")
		for _, e := range entries {
			if e.Name() == ".DS_Store" {
				continue
			}
			if e.IsDir() {
				listing.WriteString("📁 " + e.Name() + "/\n")
			} else {
				listing.WriteString("📄 " + e.Name() + "\n")
			}
		}
		return listing.String(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
