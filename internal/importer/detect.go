package importer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// Format describes which repo layout was detected.
type Format int

const (
	FormatMarketplace  Format = iota // .claude-plugin/marketplace.json
	FormatCodex                      // skills/.curated/ or skills/.system/
	FormatSingleSkill                // root SKILL.md with frontmatter
	FormatFlatSkills                 // */SKILL.md one level deep
	FormatNestedSkills               // */*/SKILL.md two levels deep
	FormatBareMarkdown               // *.md without frontmatter (non-README)
	FormatUnknown                    // not a skill repo
)

func (f Format) String() string {
	switch f {
	case FormatMarketplace:
		return "marketplace.json"
	case FormatCodex:
		return "codex (.curated/.system)"
	case FormatSingleSkill:
		return "single skill"
	case FormatFlatSkills:
		return "flat skill directories"
	case FormatNestedSkills:
		return "nested skill directories"
	case FormatBareMarkdown:
		return "bare markdown"
	default:
		return "unknown"
	}
}

// Detection holds the result of scanning a directory.
type Detection struct {
	Format      Format
	Skills      []Skill
	RootDir     string   // absolute path
	MemberGlobs []string // workspace member patterns (e.g., ["skills/*"])
}

// DetectLayout scans a directory and identifies the skill repo format.
func DetectLayout(dir string) (*Detection, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	det := &Detection{RootDir: absDir}

	// Priority 1: marketplace.json
	mpPath := filepath.Join(absDir, ".claude-plugin", "marketplace.json")
	if FileExistsAt(mpPath) {
		skills, globs, err := detectFromMarketplace(absDir, mpPath)
		if err != nil {
			return nil, fmt.Errorf("parse marketplace.json: %w", err)
		}
		det.Format = FormatMarketplace
		det.Skills = skills
		det.MemberGlobs = globs
		return det, nil
	}

	// Priority 2: Codex format
	curatedDir := filepath.Join(absDir, "skills", ".curated")
	systemDir := filepath.Join(absDir, "skills", ".system")
	if DirExistsAt(curatedDir) || DirExistsAt(systemDir) {
		skills := ScanSkillDirs(absDir, "skills/.curated")
		skills = append(skills, ScanSkillDirs(absDir, "skills/.system")...)
		det.Format = FormatCodex
		det.Skills = skills
		var memberGlobs []string
		if DirExistsAt(curatedDir) {
			memberGlobs = append(memberGlobs, "skills/.curated/*")
		}
		if DirExistsAt(systemDir) {
			memberGlobs = append(memberGlobs, "skills/.system/*")
		}
		det.MemberGlobs = memberGlobs
		return det, nil
	}

	// Priority 3: Root SKILL.md with frontmatter
	rootSkill := filepath.Join(absDir, "SKILL.md")
	if FileExistsAt(rootSkill) {
		skill, hasFM := ParseSkillAt(absDir, ".", "SKILL.md")
		if hasFM {
			if skill.Name == "" || skill.Name == "." {
				skill.Name = Slugify(filepath.Base(absDir))
			}
			det.Format = FormatSingleSkill
			det.Skills = []Skill{skill}
			return det, nil
		}
	}

	// Priority 4: Flat */SKILL.md (one level)
	flatSkills := DeduplicateSkills(ScanSkillGlob(absDir, "*/SKILL.md"))
	if len(flatSkills) > 0 {
		if len(flatSkills) == 1 {
			det.Format = FormatSingleSkill
			det.Skills = flatSkills
			return det, nil
		}
		det.Format = FormatFlatSkills
		det.Skills = flatSkills
		det.MemberGlobs = InferMemberGlobs(flatSkills)
		return det, nil
	}

	// Priority 5: Nested */*/SKILL.md (two levels)
	nestedSkills := DeduplicateSkills(ScanSkillGlob(absDir, "*/*/SKILL.md"))
	if len(nestedSkills) > 0 {
		if len(nestedSkills) == 1 {
			det.Format = FormatSingleSkill
			det.Skills = nestedSkills
			return det, nil
		}
		det.Format = FormatNestedSkills
		det.Skills = nestedSkills
		det.MemberGlobs = InferMemberGlobs(nestedSkills)
		return det, nil
	}

	// Priority 6: Bare markdown (non-README *.md without frontmatter)
	bareSkills := ScanBareMarkdown(absDir)
	if len(bareSkills) > 0 {
		det.Format = FormatBareMarkdown
		det.Skills = bareSkills
		return det, nil
	}

	// Priority 7: Unknown
	det.Format = FormatUnknown
	return det, nil
}

func detectFromMarketplace(rootDir, mpPath string) ([]Skill, []string, error) {
	f, err := os.Open(mpPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	mf, err := manifest.ParseMarketplaceJSON(f)
	if err != nil {
		return nil, nil, err
	}

	paths := manifest.MarketplaceSkillPaths(mf)
	var skills []Skill
	for _, relPath := range paths {
		absPath := filepath.Join(rootDir, relPath)
		if !DirExistsAt(absPath) {
			continue
		}
		skill, _ := ParseSkillAt(rootDir, relPath, "SKILL.md")
		if skill.Name == "" {
			skill.Name = Slugify(filepath.Base(relPath))
		}
		skill.Dir = relPath
		skills = append(skills, skill)
	}

	globs := InferMemberGlobs(skills)
	return skills, globs, nil
}
