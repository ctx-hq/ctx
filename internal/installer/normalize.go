package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// SourceFormat identifies the origin format of a SKILL.md file.
type SourceFormat string

const (
	FormatCtxNative SourceFormat = "ctx-native"
	FormatGitHubRaw SourceFormat = "github-raw"
	FormatClawHub   SourceFormat = "clawhub"
	FormatSkillsGate SourceFormat = "skillsgate"
	FormatUnknown   SourceFormat = "unknown"
)

// EnrichmentResult records what was detected and enriched.
type EnrichmentResult struct {
	SourceFormat    SourceFormat      `json:"source_format"`
	OriginalHash    string            `json:"original_hash"`
	AddedFields     map[string]string `json:"added_fields"`
	MappedFields    map[string]string `json:"mapped_fields"`
	NeedsEnrichment bool              `json:"needs_enrichment"`
}

// DetectFormat detects the source format of SKILL.md content.
func DetectFormat(content string) SourceFormat {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return FormatGitHubRaw
	}

	if strings.HasPrefix(trimmed, "---") {
		idx := strings.Index(trimmed[3:], "---")
		if idx > 0 {
			fm := trimmed[3 : 3+idx]
			if strings.Contains(fm, "metadata:") && strings.Contains(fm, "openclaw:") {
				return FormatClawHub
			}
			if strings.Contains(fm, "categories:") && strings.Contains(fm, "capabilities:") {
				return FormatSkillsGate
			}
			if strings.Contains(fm, "name:") {
				return FormatCtxNative
			}
		}
		return FormatUnknown
	}

	return FormatGitHubRaw
}

// ExtractFromMarkdown extracts name and description from raw markdown.
func ExtractFromMarkdown(content string) (name, description string, triggers []string) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if name == "" && strings.HasPrefix(trimmed, "# ") {
			name = strings.TrimSpace(trimmed[2:])
			continue
		}
		if name != "" && description == "" && len(trimmed) > 0 && !strings.HasPrefix(trimmed, "#") {
			description = trimmed
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && len(triggers) < 5 {
			heading := strings.ToLower(strings.TrimSpace(trimmed[3:]))
			if len(heading) > 2 && len(heading) < 30 {
				triggers = append(triggers, heading)
			}
		}
	}
	return
}

// Normalize detects the format and generates enrichment metadata.
func Normalize(content string) *EnrichmentResult {
	format := DetectFormat(content)

	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	if format == FormatCtxNative {
		return &EnrichmentResult{
			SourceFormat:    format,
			OriginalHash:    hashStr,
			AddedFields:     map[string]string{},
			MappedFields:    map[string]string{},
			NeedsEnrichment: false,
		}
	}

	added := map[string]string{}
	mapped := map[string]string{}

	if format == FormatGitHubRaw {
		name, desc, triggers := ExtractFromMarkdown(content)
		if name != "" {
			added["name"] = name
		}
		if desc != "" {
			added["description"] = desc
		}
		if len(triggers) > 0 {
			added["triggers"] = strings.Join(triggers, ", ")
		}
	}

	// Default compatibility for all non-native formats
	added["compatibility"] = "claude,cursor,windsurf,codex,copilot,cline,zed"

	return &EnrichmentResult{
		SourceFormat:    format,
		OriginalHash:    hashStr,
		AddedFields:     added,
		MappedFields:    mapped,
		NeedsEnrichment: len(added) > 0,
	}
}

// MergeEnrichment creates a merged SKILL.md with enriched frontmatter.
func MergeEnrichment(originalContent string, enrichment *EnrichmentResult) string {
	if !enrichment.NeedsEnrichment {
		return originalContent
	}

	keys := make([]string, 0, len(enrichment.AddedFields))
	for key := range enrichment.AddedFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var fm strings.Builder
	fm.WriteString("---\n")
	for _, key := range keys {
		fm.WriteString(key)
		fm.WriteString(": ")
		fm.WriteString(enrichment.AddedFields[key])
		fm.WriteString("\n")
	}
	fm.WriteString("---\n\n")

	// If original has frontmatter, strip it and use enriched one
	if strings.HasPrefix(originalContent, "---") {
		end := strings.Index(originalContent[3:], "---")
		if end > 0 {
			body := originalContent[3+end+3:]
			return fm.String() + strings.TrimLeft(body, "\n")
		}
	}

	return fm.String() + originalContent
}
