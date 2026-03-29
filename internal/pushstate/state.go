package pushstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
)

const stateFileName = "push-state.json"

// PushState tracks which skills have been pushed and their content hashes.
type PushState struct {
	Version int                    `json:"version"`
	Skills  map[string]*SkillState `json:"skills"`
}

// SkillState tracks push history for a single skill.
type SkillState struct {
	LastPushedHash string    `json:"last_pushed_hash"`
	LastPushedAt   time.Time `json:"last_pushed_at"`
	LastVersion    string    `json:"last_version"`
	SkillDir       string    `json:"skill_dir"`
}

// StateFile returns the path to ~/.ctx/push-state.json.
func StateFile() string {
	return filepath.Join(config.Dir(), stateFileName)
}

// Load reads push-state.json. Returns empty state if the file is missing or corrupted.
func Load() (*PushState, error) {
	ps := &PushState{Version: 1, Skills: make(map[string]*SkillState)}

	data, err := os.ReadFile(StateFile())
	if err != nil {
		if os.IsNotExist(err) {
			// Missing file is normal — return empty state.
			return ps, nil
		}
		return nil, fmt.Errorf("read push state: %w", err)
	}

	if err := json.Unmarshal(data, ps); err != nil {
		// Corrupted file — return empty state rather than failing.
		return &PushState{Version: 1, Skills: make(map[string]*SkillState)}, nil
	}

	if ps.Skills == nil {
		ps.Skills = make(map[string]*SkillState)
	}
	return ps, nil
}

// Save writes push-state.json atomically (write to temp, rename).
func (ps *PushState) Save() error {
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal push state: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(StateFile())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "push-state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp state: %w", err)
	}

	if err := os.Rename(tmpPath, StateFile()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename push state: %w", err)
	}
	return nil
}

// RecordPush updates state for a skill after a successful push.
func (ps *PushState) RecordPush(fullName, hash, version, skillDir string) {
	ps.Skills[fullName] = &SkillState{
		LastPushedHash: hash,
		LastPushedAt:   time.Now().UTC(),
		LastVersion:    version,
		SkillDir:       skillDir,
	}
}

// IsDirty returns true if the skill has never been pushed or its content has changed.
func (ps *PushState) IsDirty(fullName, currentHash string) bool {
	s, ok := ps.Skills[fullName]
	if !ok {
		return true
	}
	return s.LastPushedHash != currentHash
}

// ignoredNames are files/directories excluded from content hashing.
var ignoredNames = map[string]bool{
	".git":            true,
	".DS_Store":       true,
	"node_modules":    true,
	"package.tar.gz":  true,
	"Thumbs.db":       true,
	".ctx-managed":    true,
}

// HashDir computes a deterministic SHA-256 content hash for a skill directory.
// Files are walked in sorted order; ignored files are excluded.
func HashDir(dir string) (string, error) {
	var paths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}

		base := filepath.Base(path)

		// Skip ignored directories entirely.
		if info.IsDir() {
			if ignoredNames[base] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored files and backup files.
		if ignoredNames[base] || strings.HasSuffix(base, ".bak") {
			return nil
		}

		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk directory: %w", err)
	}

	sort.Strings(paths)

	h := sha256.New()
	for _, rel := range paths {
		// Write the relative path as part of the hash so renames are detected.
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})

		f, fErr := os.Open(filepath.Join(dir, rel))
		if fErr != nil {
			return "", fmt.Errorf("open %s: %w", rel, fErr)
		}
		if _, copyErr := io.Copy(h, f); copyErr != nil {
			_ = f.Close()
			return "", fmt.Errorf("hash %s: %w", rel, copyErr)
		}
		_ = f.Close()

		// Separator between files.
		_, _ = h.Write([]byte{0})
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
