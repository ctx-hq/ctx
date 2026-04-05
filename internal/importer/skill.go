package importer

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// Skill represents a single detected skill.
type Skill struct {
	Name           string
	Description    string
	Dir            string // relative path from root
	Entry          string // relative to skill dir, default "SKILL.md"
	Version        string
	Tags           []string
	HasFrontmatter bool
}

// ParseSkillAt reads a SKILL.md at rootDir/relDir/entry and extracts metadata.
func ParseSkillAt(rootDir, relDir, entry string) (Skill, bool) {
	fullPath := filepath.Join(rootDir, relDir, entry)
	f, err := os.Open(fullPath)
	if err != nil {
		return Skill{Dir: relDir, Entry: entry}, false
	}
	defer func() { _ = f.Close() }()

	fm, _, err := manifest.ParseSkillMD(f)
	if err != nil || fm == nil {
		return Skill{
			Name:  Slugify(filepath.Base(relDir)),
			Dir:   relDir,
			Entry: entry,
		}, false
	}

	name := fm.Name
	if name == "" {
		name = Slugify(filepath.Base(relDir))
	} else {
		name = Slugify(name)
	}

	return Skill{
		Name:           name,
		Description:    fm.Description,
		Dir:            relDir,
		Entry:          entry,
		Tags:           fm.Triggers,
		HasFrontmatter: true,
	}, true
}

// ScanSkillGlob finds skills matching a glob pattern relative to rootDir.
func ScanSkillGlob(rootDir, pattern string) []Skill {
	matches, _ := filepath.Glob(filepath.Join(rootDir, pattern))

	var skills []Skill
	for _, match := range matches {
		skillDir := filepath.Dir(match)
		relDir, _ := filepath.Rel(rootDir, skillDir)

		if ContainsExcludedDir(relDir) {
			continue
		}

		base := filepath.Base(skillDir)
		skill, _ := ParseSkillAt(rootDir, relDir, "SKILL.md")
		if skill.Name == "" {
			skill.Name = Slugify(base)
		}
		skill.Dir = relDir
		skills = append(skills, skill)
	}
	return skills
}

// ScanSkillDirs scans rootDir/prefix/*/SKILL.md for skills.
func ScanSkillDirs(rootDir, prefix string) []Skill {
	pattern := filepath.Join(rootDir, prefix, "*", "SKILL.md")
	matches, _ := filepath.Glob(pattern)

	var skills []Skill
	for _, match := range matches {
		skillDir := filepath.Dir(match)
		relDir, _ := filepath.Rel(rootDir, skillDir)
		skill, _ := ParseSkillAt(rootDir, relDir, "SKILL.md")
		if skill.Name == "" {
			skill.Name = Slugify(filepath.Base(skillDir))
		}
		skill.Dir = relDir
		skills = append(skills, skill)
	}
	return skills
}

// ScanBareMarkdown looks for non-frontmatter .md files (not README etc.) in rootDir.
func ScanBareMarkdown(rootDir string) []Skill {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		lower := strings.ToLower(name)
		if lower == "readme.md" || lower == "changelog.md" || lower == "contributing.md" ||
			lower == "license.md" || lower == "claude.md" || lower == "spec.md" ||
			lower == "tasks.md" || lower == "description.md" {
			continue
		}

		f, err := os.Open(filepath.Join(rootDir, name))
		if err != nil {
			continue
		}
		fm, _, _ := manifest.ParseSkillMD(f)
		_ = f.Close()

		if fm != nil {
			continue
		}

		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		skills = append(skills, Skill{
			Name:  Slugify(baseName),
			Dir:   ".",
			Entry: name,
		})
		break // only import the first bare markdown to avoid pulling in unrelated docs
	}
	return skills
}

// DeduplicateSkills removes skills with the same name, keeping the first occurrence.
func DeduplicateSkills(skills []Skill) []Skill {
	seen := make(map[string]bool)
	var unique []Skill
	for _, s := range skills {
		if !seen[s.Name] {
			seen[s.Name] = true
			unique = append(unique, s)
		}
	}
	return unique
}

// InferMemberGlobs determines the workspace member glob patterns from detected skills.
func InferMemberGlobs(skills []Skill) []string {
	if len(skills) == 0 {
		return []string{"*"}
	}

	type prefixInfo struct {
		prefix   string
		twoLevel int
		total    int
		order    int
	}
	prefixes := make(map[string]*prefixInfo)
	insertOrder := 0

	for _, s := range skills {
		parts := strings.SplitN(s.Dir, "/", 2)
		var key string
		if len(parts) >= 2 {
			key = parts[0]
		}
		info, ok := prefixes[key]
		if !ok {
			info = &prefixInfo{prefix: key, order: insertOrder}
			insertOrder++
			prefixes[key] = info
		}
		info.total++
		if strings.Count(s.Dir, "/") >= 2 {
			info.twoLevel++
		}
	}

	if len(prefixes) == 1 {
		for k := range prefixes {
			if k == "" || k == "." {
				return []string{"*"}
			}
		}
	}

	sorted := make([]*prefixInfo, 0, len(prefixes))
	for _, info := range prefixes {
		sorted = append(sorted, info)
	}
	slices.SortFunc(sorted, func(a, b *prefixInfo) int {
		return cmp.Compare(a.order, b.order)
	})

	var globs []string
	for _, info := range sorted {
		if info.prefix == "" || info.prefix == "." {
			globs = append(globs, "*")
		} else if info.twoLevel > info.total/2 {
			globs = append(globs, info.prefix+"/*/*")
		} else {
			globs = append(globs, info.prefix+"/*")
		}
	}
	return globs
}
