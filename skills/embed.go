// Package skills embeds the ctx SKILL.md file in the binary.
package skills

import "embed"

//go:embed ctx
var FS embed.FS
