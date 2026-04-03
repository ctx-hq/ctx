package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoProfile indicates no profile is configured.
var ErrNoProfile = errors.New("no profile configured")

// ResolveResult contains the fully resolved profile with provenance info.
type ResolveResult struct {
	Name    string   `json:"name"`
	Profile *Profile `json:"profile"`
	Source  string   `json:"source"` // "flag", "env", "project", "global", "default"
}

// Resolve returns the active profile by walking the priority chain.
// flagProfile is the value of the --profile flag (pass "" if not set).
//
// Resolution order (SSOT):
//  1. --profile flag
//  2. CTX_PROFILE env var
//  3. .ctx-profile file (walk up from CWD)
//  4. profiles.yaml active field
//  5. "default" profile name
//  6. ErrNoProfile
func Resolve(flagProfile string) (*ResolveResult, error) {
	store, err := Load()
	if err != nil {
		return nil, err
	}
	return resolveFromStore(store, flagProfile)
}

// resolveFromStore resolves a profile from an already-loaded store.
// Extracted for testability (avoids disk I/O in tests).
func resolveFromStore(store *ProfileStore, flagProfile string) (*ResolveResult, error) {
	// ① --profile flag
	if flagProfile != "" {
		p, ok := store.Profiles[flagProfile]
		if !ok {
			return nil, profileNotFound(flagProfile, "--profile flag")
		}
		return &ResolveResult{Name: flagProfile, Profile: p, Source: "flag"}, nil
	}

	// ② CTX_PROFILE env var
	if env := os.Getenv("CTX_PROFILE"); env != "" {
		p, ok := store.Profiles[env]
		if !ok {
			return nil, profileNotFound(env, "CTX_PROFILE environment variable")
		}
		return &ResolveResult{Name: env, Profile: p, Source: "env"}, nil
	}

	// ③ .ctx-profile file (walk up from CWD)
	if name := FindProjectProfile(); name != "" {
		p, ok := store.Profiles[name]
		if !ok {
			return nil, profileNotFound(name, ".ctx-profile")
		}
		return &ResolveResult{Name: name, Profile: p, Source: "project"}, nil
	}

	// ④ profiles.yaml active
	if store.Active != "" {
		if p, ok := store.Profiles[store.Active]; ok {
			return &ResolveResult{Name: store.Active, Profile: p, Source: "global"}, nil
		}
		// active references a deleted profile — fall through
	}

	// ⑤ "default" fallback
	if p, ok := store.Profiles["default"]; ok {
		return &ResolveResult{Name: "default", Profile: p, Source: "default"}, nil
	}

	// ⑥ No profiles
	return nil, ErrNoProfile
}

// FindProjectProfile walks up from CWD looking for .ctx-profile.
// Returns the profile name, or "" if no file is found.
func FindProjectProfile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return findProjectProfileFrom(dir)
}

// findProjectProfileFrom walks up from the given directory looking for .ctx-profile.
func findProjectProfileFrom(dir string) string {
	for {
		data, err := os.ReadFile(filepath.Join(dir, ".ctx-profile"))
		if err == nil {
			name := strings.TrimSpace(string(data))
			// Must be a single, non-empty line
			if name != "" && !strings.Contains(name, "\n") {
				return name
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// profileNotFound returns an error indicating the named profile does not exist.
func profileNotFound(name, source string) error {
	return fmt.Errorf("profile %q not found (referenced by %s)\n  Hint: run 'ctx profile list' to see available profiles", name, source)
}
