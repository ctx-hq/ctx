package manifest

import (
	"bufio"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillFrontmatter represents the YAML frontmatter of a SKILL.md file.
type SkillFrontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Triggers     []string `yaml:"triggers"`
	Invocable    bool     `yaml:"invocable"`
	ArgumentHint string   `yaml:"argument-hint"`
}

// ParseSkillMD parses a SKILL.md file, extracting frontmatter and body.
// Returns (frontmatter, body, error). If no frontmatter is found,
// returns nil frontmatter with the full content as body.
func ParseSkillMD(r io.Reader) (*SkillFrontmatter, string, error) {
	scanner := bufio.NewScanner(r)

	// Check for opening ---
	if !scanner.Scan() {
		return nil, "", nil // empty file
	}
	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		// No frontmatter — return full content as body
		var body strings.Builder
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
		for scanner.Scan() {
			body.WriteString(scanner.Text())
			body.WriteByte('\n')
		}
		return nil, body.String(), nil
	}

	// Read frontmatter until closing ---
	var fmLines strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines.WriteString(line)
		fmLines.WriteByte('\n')
	}

	// Parse frontmatter YAML
	var fm SkillFrontmatter
	if err := yaml.Unmarshal([]byte(fmLines.String()), &fm); err != nil {
		return nil, "", err
	}

	// Read remaining body
	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}

	return &fm, body.String(), nil
}

// ValidateSkillMD cross-validates a SKILL.md frontmatter against the ctx.yaml manifest.
// Returns a list of warnings (not errors — SKILL.md validation is advisory).
func ValidateSkillMD(fm *SkillFrontmatter, m *Manifest) []string {
	var warnings []string

	if fm == nil {
		warnings = append(warnings, "SKILL.md has no frontmatter (recommended for agent discovery)")
		return warnings
	}

	// Name consistency
	if fm.Name != "" && m != nil {
		shortName := m.ShortName()
		if fm.Name != shortName {
			warnings = append(warnings, "SKILL.md name "+fm.Name+" differs from ctx.yaml short name "+shortName)
		}
	}

	// Required fields
	if fm.Name == "" {
		warnings = append(warnings, "SKILL.md frontmatter missing 'name' field")
	}
	if fm.Description == "" {
		warnings = append(warnings, "SKILL.md frontmatter missing 'description' field")
	}

	// Recommended fields
	if len(fm.Triggers) == 0 {
		warnings = append(warnings, "SKILL.md has no triggers (agents may not auto-invoke this skill)")
	} else if len(fm.Triggers) < 3 {
		warnings = append(warnings, "SKILL.md has few triggers (recommend at least 3 for better agent discovery)")
	}

	if fm.Invocable && fm.ArgumentHint == "" {
		warnings = append(warnings, "SKILL.md is invocable but missing 'argument-hint'")
	}

	return warnings
}
