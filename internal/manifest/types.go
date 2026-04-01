package manifest

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PackageType represents the type of ctx package.
type PackageType string

const (
	TypeSkill      PackageType = "skill"
	TypeMCP        PackageType = "mcp"
	TypeCLI        PackageType = "cli"
	TypeWorkspace  PackageType = "workspace"
	TypeCollection PackageType = "collection"
)

func (t PackageType) Valid() bool {
	return t == TypeSkill || t == TypeMCP || t == TypeCLI || t == TypeWorkspace || t == TypeCollection
}

// Manifest represents a parsed ctx.yaml file.
type Manifest struct {
	Name        string      `yaml:"name" json:"name"`
	Version     string      `yaml:"version,omitempty" json:"version,omitempty"`
	Type        PackageType `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Author      string      `yaml:"author,omitempty" json:"author,omitempty"`
	License     string      `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage    string      `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository  string      `yaml:"repository,omitempty" json:"repository,omitempty"`
	Keywords    []string    `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	Skill *SkillSpec `yaml:"skill,omitempty" json:"skill,omitempty"`
	MCP   *MCPSpec   `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	CLI   *CLISpec   `yaml:"cli,omitempty" json:"cli,omitempty"`

	Workspace  *WorkspaceSpec      `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Collection *CollectionManifest `yaml:"collection,omitempty" json:"collection,omitempty"`

	Source       *SourceSpec        `yaml:"source,omitempty" json:"source,omitempty"`
	Install      *InstallSpec      `yaml:"install,omitempty" json:"install,omitempty"`
	Dependencies map[string]string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	Visibility string `yaml:"visibility,omitempty" json:"visibility,omitempty"` // public, unlisted, private
	Mutable    bool   `yaml:"mutable,omitempty" json:"mutable,omitempty"`       // allow version overwrite (private only)
}

// SourceSpec describes an external source for adapter packages (Homebrew formula pattern).
type SourceSpec struct {
	GitHub string `yaml:"github,omitempty" json:"github,omitempty"` // owner/repo
	Path   string `yaml:"path,omitempty" json:"path,omitempty"`     // path within repo
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`       // tag, branch, or commit
}

// Scope returns the @scope part of the name.
func (m *Manifest) Scope() string {
	scope, _ := ParseFullName(m.Name)
	return scope
}

// ShortName returns the name part without the scope.
func (m *Manifest) ShortName() string {
	_, name := ParseFullName(m.Name)
	return name
}

// SkillSpec contains skill-type configuration.
type SkillSpec struct {
	Entry         string   `yaml:"entry,omitempty" json:"entry,omitempty"`
	Compatibility string   `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	Tags          []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	UserInvocable *bool    `yaml:"user_invocable,omitempty" json:"user_invocable,omitempty"`
	Origin        string   `yaml:"origin,omitempty" json:"origin,omitempty"` // "native" (from source repo) or "wrapped" (generated)
}

// MCPSpec contains MCP server configuration.
type MCPSpec struct {
	Transport string            `yaml:"transport" json:"transport"`
	Command   string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env       []EnvVar          `yaml:"env,omitempty" json:"env,omitempty"`
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Tools     []string          `yaml:"tools,omitempty" json:"tools,omitempty"`
	Resources []string          `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// EnvVar represents an environment variable declaration.
type EnvVar struct {
	Name        string `yaml:"name" json:"name"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// CLISpec contains CLI tool configuration.
type CLISpec struct {
	Binary     string       `yaml:"binary" json:"binary"`
	Verify     string       `yaml:"verify,omitempty" json:"verify,omitempty"`
	Compatible string       `yaml:"compatible,omitempty" json:"compatible,omitempty"` // semver range for CLI version
	Require    *RequireSpec `yaml:"require,omitempty" json:"require,omitempty"`
	Auth       string       `yaml:"auth,omitempty" json:"auth,omitempty"` // human-readable auth/setup hint
}

// RequireSpec declares runtime prerequisites.
type RequireSpec struct {
	Bins []string `yaml:"bins,omitempty" json:"bins,omitempty"`
	Env  []string `yaml:"env,omitempty" json:"env,omitempty"`
}

// InstallSpec describes how to obtain and install the package (formula/bridge layer).
type InstallSpec struct {
	Source    string                      `yaml:"source,omitempty" json:"source,omitempty"`
	Brew      string                      `yaml:"brew,omitempty" json:"brew,omitempty"`
	Npm       string                      `yaml:"npm,omitempty" json:"npm,omitempty"`
	Pip       string                      `yaml:"pip,omitempty" json:"pip,omitempty"`
	Gem       string                      `yaml:"gem,omitempty" json:"gem,omitempty"`
	Cargo     string                      `yaml:"cargo,omitempty" json:"cargo,omitempty"`
	Script    string                      `yaml:"script,omitempty" json:"script,omitempty"` // shell script URL (curl|sh style)
	Platforms map[string]*PlatformInstall `yaml:"platforms,omitempty" json:"platforms,omitempty"`
}

// PlatformInstall provides platform-specific install overrides.
type PlatformInstall struct {
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	Brew   string `yaml:"brew,omitempty" json:"brew,omitempty"`
	Npm    string `yaml:"npm,omitempty" json:"npm,omitempty"`
	Binary string `yaml:"binary,omitempty" json:"binary,omitempty"`
}

// WorkspaceSpec defines a monorepo workspace containing multiple packages.
type WorkspaceSpec struct {
	Members     []string           `yaml:"members" json:"members"`                           // glob patterns: ["skills/*", "engineering/*"]
	Exclude     []string           `yaml:"exclude,omitempty" json:"exclude,omitempty"`        // exclude patterns: ["docs", "scripts"]
	Defaults    *WorkspaceDefaults `yaml:"defaults,omitempty" json:"defaults,omitempty"`      // inherited by members
	Collections []CollectionSpec   `yaml:"collections,omitempty" json:"collections,omitempty"` // named sub-groups
}

// WorkspaceDefaults holds default values inherited by workspace members.
type WorkspaceDefaults struct {
	Scope      string `yaml:"scope,omitempty" json:"scope,omitempty"`
	Author     string `yaml:"author,omitempty" json:"author,omitempty"`
	License    string `yaml:"license,omitempty" json:"license,omitempty"`
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`
}

// CollectionSpec defines a named sub-group of workspace members.
type CollectionSpec struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Members     []string `yaml:"members" json:"members"` // skill short names or globs
}

// CollectionManifest is the manifest for a published collection package.
type CollectionManifest struct {
	Members []string `yaml:"members" json:"members"` // full names: ["@scope/skill1", "@scope/skill2"]
}

// PackageFiles returns the list of files/directories that should be included
// in a published archive. Only these paths (relative to the project root)
// are packaged — everything else (source code, build artifacts, etc.) is excluded.
func (m *Manifest) PackageFiles() []string {
	files := []string{"ctx.yaml", "README.md"}

	// LICENSE candidates — CopyFiles silently skips missing paths.
	files = append(files,
		"LICENSE", "LICENSE.md", "LICENSE.txt",
		"LICENCE", "LICENCE.md", "LICENCE.txt",
	)

	if m.Skill != nil && m.Skill.Entry != "" {
		entry := filepath.Clean(m.Skill.Entry)
		// Include the skill entry's parent directory (contains SKILL.md + scripts/, references/, assets/)
		dir := filepath.Dir(entry)
		if dir != "." {
			files = append(files, dir)
		} else {
			// Skill at root — include actual entry file, conventional subdirectories,
			// and known .md files. (README.md and LICENSE variants already added above.)
			files = append(files, entry, "scripts", "references", "assets", "agents",
				"EXTEND.md")
		}
	}

	// MCP stdio packages may ship local runtime files (e.g. node dist/index.js).
	if m.MCP != nil && m.MCP.Transport == "stdio" {
		for _, arg := range m.MCP.Args {
			if arg == "" || strings.HasPrefix(arg, "-") {
				continue
			}
			dir := filepath.Dir(filepath.Clean(arg))
			if dir != "." {
				files = append(files, dir)
			} else {
				files = append(files, arg)
			}
		}
	}

	return files
}

// SkillEntryNeedsNormalize reports whether the skill entry should be copied
// to a root SKILL.md for install-side compatibility.
func (m *Manifest) SkillEntryNeedsNormalize() bool {
	return m.Skill != nil && m.Skill.Entry != "" && m.Skill.Entry != "SKILL.md"
}

// ParseFullName splits "@scope/name" into ("scope", "name").
func ParseFullName(fullName string) (scope, name string) {
	if len(fullName) == 0 {
		return "", ""
	}
	s := fullName
	if s[0] == '@' {
		s = s[1:]
	}
	for i, c := range s {
		if c == '/' {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

// FormatFullName formats scope and name into "@scope/name".
func FormatFullName(scope, name string) string {
	if scope == "" {
		return name
	}
	return fmt.Sprintf("@%s/%s", scope, name)
}
