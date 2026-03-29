package manifest

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// RenderSkillMD serializes frontmatter and body into SKILL.md format.
// If fm is nil, returns body as-is (no frontmatter injected).
func RenderSkillMD(fm *SkillFrontmatter, body string) ([]byte, error) {
	if fm == nil {
		return []byte(body), nil
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n")

	// Add blank line between frontmatter and body if body doesn't start with one
	if body != "" && body[0] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(body)

	return buf.Bytes(), nil
}
