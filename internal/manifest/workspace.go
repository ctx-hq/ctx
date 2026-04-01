package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Workspace holds the resolved workspace state with root manifest and members.
type Workspace struct {
	Root    *Manifest
	RootDir string
	Members []*WorkspaceMember
}

// WorkspaceMember represents a resolved workspace member.
type WorkspaceMember struct {
	Dir      string    // absolute path to the member directory
	RelDir   string    // relative path from workspace root
	Manifest *Manifest // parsed manifest (from ctx.yaml or auto-scaffold)
	Source   string    // "ctx.yaml" or "auto-scaffold"
}

// LoadWorkspace loads a workspace root manifest and resolves all member manifests.
func LoadWorkspace(rootDir string) (*Workspace, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	root, err := LoadFromDir(absRoot)
	if err != nil {
		return nil, fmt.Errorf("load workspace manifest: %w", err)
	}

	if root.Type != TypeWorkspace {
		return nil, fmt.Errorf("manifest type is %q, expected workspace", root.Type)
	}

	if root.Workspace == nil {
		return nil, fmt.Errorf("workspace section is missing")
	}

	members, err := resolveWorkspaceMembers(absRoot, root.Workspace)
	if err != nil {
		return nil, err
	}

	// Apply defaults to members that need it.
	if root.Workspace.Defaults != nil {
		for _, m := range members {
			ApplyDefaults(m.Manifest, root.Workspace.Defaults)
		}
	}

	// Validate no duplicate names.
	if err := validateUniqueNames(members); err != nil {
		return nil, err
	}

	return &Workspace{
		Root:    root,
		RootDir: absRoot,
		Members: members,
	}, nil
}

// ResolveMembers expands glob patterns against rootDir and returns matching
// directories that contain either a ctx.yaml or SKILL.md file.
// Exclude patterns filter out directories by base name.
func ResolveMembers(rootDir string, patterns, exclude []string) ([]string, error) {
	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}
	// Always exclude common non-skill dirs.
	for _, d := range []string{".git", "node_modules", ".github"} {
		excludeSet[d] = true
	}

	seen := make(map[string]bool)
	var dirs []string

	for _, pattern := range patterns {
		fullPattern := filepath.Join(rootDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}

		for _, match := range matches {
			info, statErr := os.Stat(match)
			if statErr != nil || !info.IsDir() {
				continue
			}

			base := filepath.Base(match)
			if excludeSet[base] {
				continue
			}

			// Require either ctx.yaml or SKILL.md to qualify as a member.
			hasCtxYaml := fileExists(filepath.Join(match, FileName))
			hasSkillMD := fileExists(filepath.Join(match, "SKILL.md"))
			if !hasCtxYaml && !hasSkillMD {
				continue
			}

			// Check for nested workspace (not supported).
			if hasCtxYaml {
				m, loadErr := LoadFromFile(filepath.Join(match, FileName))
				if loadErr == nil && m.Type == TypeWorkspace {
					return nil, fmt.Errorf("nested workspace at %q is not supported", match)
				}
			}

			if !seen[match] {
				seen[match] = true
				dirs = append(dirs, match)
			}
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

// ApplyDefaults merges workspace defaults into a member manifest.
// Member values (explicit) always take precedence over defaults.
func ApplyDefaults(member *Manifest, defaults *WorkspaceDefaults) {
	if defaults == nil {
		return
	}

	// Apply scope: if member name has no scope, prefix with default scope.
	if defaults.Scope != "" {
		scope, _ := ParseFullName(member.Name)
		if scope == "" && member.Name != "" {
			scopeBase := strings.TrimPrefix(defaults.Scope, "@")
			shortName := member.Name
			// Strip scope prefix from name to avoid duplication.
			// e.g., scope="@baoyu" + name="baoyu-translate" → "@baoyu/translate" (not "@baoyu/baoyu-translate")
			if strings.HasPrefix(shortName, scopeBase+"-") {
				shortName = strings.TrimPrefix(shortName, scopeBase+"-")
			}
			member.Name = FormatFullName(scopeBase, shortName)
		}
	}

	if defaults.Author != "" && member.Author == "" {
		member.Author = defaults.Author
	}
	if defaults.License != "" && member.License == "" {
		member.License = defaults.License
	}
	if defaults.Repository != "" && member.Repository == "" {
		member.Repository = defaults.Repository
	}
}

// ResolveCollections maps collection specs to their resolved workspace members.
// Returns a map from collection name to the list of matching members.
func ResolveCollections(ws *Workspace) (map[string][]*WorkspaceMember, error) {
	if ws.Root.Workspace == nil || len(ws.Root.Workspace.Collections) == 0 {
		return nil, nil
	}

	// Build lookup by short name and relative dir.
	byShortName := make(map[string]*WorkspaceMember)
	byRelDir := make(map[string]*WorkspaceMember)
	for _, m := range ws.Members {
		_, shortName := ParseFullName(m.Manifest.Name)
		byShortName[shortName] = m
		byRelDir[m.RelDir] = m
	}

	result := make(map[string][]*WorkspaceMember)

	for _, col := range ws.Root.Workspace.Collections {
		var members []*WorkspaceMember
		for _, pattern := range col.Members {
			matched := matchCollectionMembers(pattern, ws.Members, byShortName, byRelDir)
			members = append(members, matched...)
		}

		if len(members) == 0 {
			return nil, fmt.Errorf("collection %q: no members matched patterns %v", col.Name, col.Members)
		}

		// Deduplicate.
		seen := make(map[string]bool)
		var deduped []*WorkspaceMember
		for _, m := range members {
			if !seen[m.Dir] {
				seen[m.Dir] = true
				deduped = append(deduped, m)
			}
		}
		result[col.Name] = deduped
	}

	return result, nil
}

// ScaffoldFromSkillMD creates a minimal manifest from a SKILL.md frontmatter.
func ScaffoldFromSkillMD(dir string) (*Manifest, error) {
	skillPath := filepath.Join(dir, "SKILL.md")
	f, err := os.Open(skillPath)
	if err != nil {
		return nil, fmt.Errorf("open SKILL.md: %w", err)
	}
	defer func() { _ = f.Close() }()

	fm, _, err := ParseSkillMD(f)
	if err != nil {
		return nil, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}

	if fm == nil {
		return nil, fmt.Errorf("SKILL.md has no frontmatter in %s", dir)
	}

	if fm.Name == "" {
		// Fall back to directory name.
		fm.Name = filepath.Base(dir)
	}

	m := &Manifest{
		Name:        fm.Name, // may be bare name; ApplyDefaults adds scope later
		Version:     "0.1.0",
		Type:        TypeSkill,
		Description: fm.Description,
		Skill:       &SkillSpec{Entry: "SKILL.md"},
	}

	if m.Description == "" {
		m.Description = fmt.Sprintf("Skill: %s", fm.Name)
	}

	// Truncate description to manifest limit (1024 chars).
	if len(m.Description) > 1024 {
		m.Description = m.Description[:1021] + "..."
	}

	return m, nil
}

// --- internal helpers ---

func resolveWorkspaceMembers(rootDir string, ws *WorkspaceSpec) ([]*WorkspaceMember, error) {
	dirs, err := ResolveMembers(rootDir, ws.Members, ws.Exclude)
	if err != nil {
		return nil, err
	}

	var members []*WorkspaceMember
	for _, dir := range dirs {
		relDir, _ := filepath.Rel(rootDir, dir)

		var m *Manifest
		var source string

		ctxYamlPath := filepath.Join(dir, FileName)
		if fileExists(ctxYamlPath) {
			loaded, loadErr := LoadFromFile(ctxYamlPath)
			if loadErr != nil {
				return nil, fmt.Errorf("load member %s: %w", relDir, loadErr)
			}
			m = loaded
			source = "ctx.yaml"
		} else {
			scaffolded, scaffoldErr := ScaffoldFromSkillMD(dir)
			if scaffoldErr != nil {
				return nil, fmt.Errorf("auto-scaffold member %s: %w", relDir, scaffoldErr)
			}
			m = scaffolded
			source = "auto-scaffold"
		}

		members = append(members, &WorkspaceMember{
			Dir:      dir,
			RelDir:   relDir,
			Manifest: m,
			Source:   source,
		})
	}

	return members, nil
}

func validateUniqueNames(members []*WorkspaceMember) error {
	seen := make(map[string]string) // name → relDir
	for _, m := range members {
		name := m.Manifest.Name
		if prev, ok := seen[name]; ok {
			return fmt.Errorf("duplicate member name %q: found in %s and %s", name, prev, m.RelDir)
		}
		seen[name] = m.RelDir
	}
	return nil
}

func matchCollectionMembers(pattern string, allMembers []*WorkspaceMember, byShortName map[string]*WorkspaceMember, byRelDir map[string]*WorkspaceMember) []*WorkspaceMember {
	// Try exact short name match first.
	if m, ok := byShortName[pattern]; ok {
		return []*WorkspaceMember{m}
	}

	// Try relative dir match.
	if m, ok := byRelDir[pattern]; ok {
		return []*WorkspaceMember{m}
	}

	// Try glob match against relative dirs, short names, and dir basenames.
	var matched []*WorkspaceMember
	for _, m := range allMembers {
		dirBase := filepath.Base(m.Dir) // e.g., "baoyu-translate"
		_, shortName := ParseFullName(m.Manifest.Name) // e.g., "translate"

		// Exact match on dir basename (handles marketplace.json patterns like "baoyu-translate").
		if pattern == dirBase {
			matched = append(matched, m)
			continue
		}

		// Glob match against relative dir.
		if ok, _ := filepath.Match(pattern, m.RelDir); ok {
			matched = append(matched, m)
			continue
		}

		// Glob match against short name.
		if ok, _ := filepath.Match(pattern, shortName); ok {
			matched = append(matched, m)
			continue
		}

		// Glob match against dir basename.
		if ok, _ := filepath.Match(pattern, dirBase); ok {
			matched = append(matched, m)
		}
	}

	return matched
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
