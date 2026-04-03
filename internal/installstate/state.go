package installstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const stateFileName = "state.json"
const stateSchemaVersion = 1

// PackageState tracks the installation state of a package for idempotency and repair.
type PackageState struct {
	SchemaVersion int          `json:"schema_version"`
	FullName      string       `json:"full_name"`
	Version       string       `json:"version"`
	Type          string       `json:"type"`
	Source        string       `json:"source,omitempty"`         // "registry", "github", "push"
	ArchiveSHA256 string       `json:"archive_sha256,omitempty"` // SHA256 of downloaded archive for integrity audit
	InstalledAt   time.Time    `json:"installed_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	CLI           *CLIState    `json:"cli,omitempty"`
	Skills        []SkillState `json:"skills,omitempty"`
	MCP           []MCPState   `json:"mcp,omitempty"`
	Hooks         *HooksState  `json:"hooks,omitempty"`
}

// CLIState tracks how a CLI binary was installed.
type CLIState struct {
	Adapter    string `json:"adapter"`      // adapter name: brew, npm, gem, script, etc.
	AdapterPkg string `json:"adapter_pkg"`  // exact string passed to adapter
	Binary     string `json:"binary"`       // binary name
	BinaryPath string `json:"binary_path"`  // resolved PATH location
	Verified   bool   `json:"verified"`
	Status     string `json:"status"` // "ok" or "failed"
}

// SkillState tracks a skill link to an agent.
type SkillState struct {
	Agent       string `json:"agent"`        // agent name: claude, cursor, etc.
	SymlinkPath string `json:"symlink_path"` // where the symlink lives
	Status      string `json:"status"`       // "ok" or "broken"
}

// MCPState tracks an MCP config entry in an agent.
type MCPState struct {
	Agent       string `json:"agent"`
	ConfigKey   string `json:"config_key"`             // key in mcpServers
	Status      string `json:"status"`                 // "ok" or "missing"
	TransportID string `json:"transport_id,omitempty"` // selected transport from transports[]
}

// HooksState tracks lifecycle hook completion.
type HooksState struct {
	PostInstall []string `json:"post_install,omitempty"` // completed hook descriptions
}

// Load reads state.json from a package directory.
// Returns nil, nil if the file does not exist.
func Load(pkgDir string) (*PackageState, error) {
	path := filepath.Join(pkgDir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state PackageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &state, nil
}

// Save writes state.json to the package directory atomically (temp + rename).
func (s *PackageState) Save(pkgDir string) error {
	s.SchemaVersion = stateSchemaVersion
	s.UpdatedAt = time.Now().UTC()

	if err := os.MkdirAll(pkgDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	path := filepath.Join(pkgDir, stateFileName)
	tmp, err := os.CreateTemp(pkgDir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp to state.json: %w", err)
	}
	return nil
}

// Remove deletes state.json from the package directory.
func Remove(pkgDir string) error {
	path := filepath.Join(pkgDir, stateFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}
	return nil
}
