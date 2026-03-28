package manifest

import "fmt"

// PackageType represents the type of ctx package.
type PackageType string

const (
	TypeSkill PackageType = "skill"
	TypeMCP   PackageType = "mcp"
	TypeCLI   PackageType = "cli"
)

func (t PackageType) Valid() bool {
	return t == TypeSkill || t == TypeMCP || t == TypeCLI
}

// Manifest represents a parsed ctx.yaml file.
type Manifest struct {
	Name        string      `yaml:"name" json:"name"`
	Version     string      `yaml:"version" json:"version"`
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

	Source       *SourceSpec        `yaml:"source,omitempty" json:"source,omitempty"`
	Install      *InstallSpec      `yaml:"install,omitempty" json:"install,omitempty"`
	Dependencies map[string]string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
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
	Cargo     string                      `yaml:"cargo,omitempty" json:"cargo,omitempty"`
	Platforms map[string]*PlatformInstall `yaml:"platforms,omitempty" json:"platforms,omitempty"`
}

// PlatformInstall provides platform-specific install overrides.
type PlatformInstall struct {
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	Brew   string `yaml:"brew,omitempty" json:"brew,omitempty"`
	Npm    string `yaml:"npm,omitempty" json:"npm,omitempty"`
	Binary string `yaml:"binary,omitempty" json:"binary,omitempty"`
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
