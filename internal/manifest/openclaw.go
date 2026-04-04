package manifest

import (
	"bufio"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenClawMetadata represents the metadata.openclaw section in a SKILL.md frontmatter.
// Also supports aliases: metadata.clawdbot, metadata.clawdis.
type OpenClawMetadata struct {
	Requires *OpenClawRequires `yaml:"requires,omitempty"`
	Install  *OpenClawInstall  `yaml:"install,omitempty"`
	Config   *OpenClawConfig   `yaml:"config,omitempty"`
}

// OpenClawRequires declares runtime prerequisites.
type OpenClawRequires struct {
	Env     []string `yaml:"env,omitempty"`
	Bins    []string `yaml:"bins,omitempty"`
	AnyBins []string `yaml:"anyBins,omitempty"`
}

// OpenClawInstall declares how to install required binaries.
type OpenClawInstall struct {
	Brew []string `yaml:"brew,omitempty"`
	Node []string `yaml:"node,omitempty"`
	Apt  []string `yaml:"apt,omitempty"`
}

// OpenClawConfig declares configuration requirements.
type OpenClawConfig struct {
	RequiredEnv []string `yaml:"requiredEnv,omitempty"`
	StateDirs   []string `yaml:"stateDirs,omitempty"`
}

// openClawFrontmatter is an extended frontmatter type for parsing OpenClaw metadata.
type openClawFrontmatter struct {
	Name        string                        `yaml:"name"`
	Description string                        `yaml:"description"`
	License     string                        `yaml:"license"`
	Version     string                        `yaml:"version"`
	Metadata    map[string]*OpenClawMetadata   `yaml:"metadata,omitempty"`
}

// ParseOpenClawFrontmatter extracts OpenClaw metadata from a SKILL.md reader.
// Returns nil metadata if no openclaw/clawdbot/clawdis key is found.
func ParseOpenClawFrontmatter(r io.Reader) (*openClawFrontmatter, error) {
	scanner := bufio.NewScanner(r)

	if !scanner.Scan() {
		return nil, nil
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, nil
	}

	var fmLines strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines.WriteString(line)
		fmLines.WriteByte('\n')
	}

	var fm openClawFrontmatter
	if err := yaml.Unmarshal([]byte(fmLines.String()), &fm); err != nil {
		return nil, err
	}

	return &fm, nil
}

// GetOpenClawMetadata returns the OpenClaw metadata from the frontmatter,
// checking openclaw, clawdbot, and clawdis keys in order of priority.
func (fm *openClawFrontmatter) GetOpenClawMetadata() *OpenClawMetadata {
	if fm == nil || fm.Metadata == nil {
		return nil
	}
	for _, key := range []string{"openclaw", "clawdbot", "clawdis"} {
		if m, ok := fm.Metadata[key]; ok {
			return m
		}
	}
	return nil
}

// OpenClawToCtx converts OpenClaw SKILL.md metadata into ctx manifest fields.
// It creates a new Manifest populated with information extracted from the
// SKILL.md frontmatter, including install specs and runtime requirements.
func OpenClawToCtx(fm *openClawFrontmatter) *Manifest {
	if fm == nil {
		return nil
	}

	m := &Manifest{
		Name:        fm.Name,
		Version:     fm.Version,
		Type:        TypeSkill,
		Description: fm.Description,
		License:     fm.License,
		Skill:       &SkillSpec{Entry: "SKILL.md"},
	}

	if m.Version == "" {
		m.Version = "0.1.0"
	}

	oc := fm.GetOpenClawMetadata()
	if oc == nil {
		return m
	}

	// Map install specs.
	if oc.Install != nil {
		install := &InstallSpec{}
		hasInstall := false
		if len(oc.Install.Brew) > 0 {
			install.Brew = oc.Install.Brew[0] // ctx supports single brew formula
			hasInstall = true
		}
		if len(oc.Install.Node) > 0 {
			install.Npm = oc.Install.Node[0]
			hasInstall = true
		}
		if hasInstall {
			m.Install = install
		}
	}

	// Map runtime requirements as keywords. ctx doesn't have a dedicated
	// requires field on SkillSpec yet, so "env:X"/"bin:X" keywords are the
	// only landing spot. These are exempt from the reserved-prefix validation
	// (which applies to hand-authored ctx.yaml, not OpenClaw imports).
	if oc.Requires != nil {
		for _, env := range oc.Requires.Env {
			m.Keywords = appendUnique(m.Keywords, "env:"+env)
		}
		for _, bin := range oc.Requires.Bins {
			m.Keywords = appendUnique(m.Keywords, "bin:"+bin)
		}
	}

	return m
}

// CtxToOpenClaw generates OpenClaw-compatible metadata from a ctx manifest.
// Returns a map suitable for embedding in SKILL.md frontmatter metadata.
func CtxToOpenClaw(m *Manifest) map[string]interface{} {
	oc := make(map[string]interface{})

	// Map install specs.
	if m.Install != nil {
		install := make(map[string]interface{})
		if m.Install.Brew != "" {
			install["brew"] = []string{m.Install.Brew}
		}
		if m.Install.Npm != "" {
			install["node"] = []string{m.Install.Npm}
		}
		if len(install) > 0 {
			oc["install"] = install
		}
	}

	// Map CLI requirements.
	if m.CLI != nil && m.CLI.Require != nil {
		requires := make(map[string]interface{})
		if len(m.CLI.Require.Bins) > 0 {
			requires["bins"] = m.CLI.Require.Bins
		}
		if len(m.CLI.Require.Env) > 0 {
			requires["env"] = m.CLI.Require.Env
		}
		if len(requires) > 0 {
			oc["requires"] = requires
		}
	}

	if len(oc) == 0 {
		return nil
	}

	return map[string]interface{}{
		"openclaw": oc,
	}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
