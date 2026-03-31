package publishcheck

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/manifest"
)

// FormatChecklist renders a pre-publish checklist for human review.
func FormatChecklist(m *manifest.Manifest, results []CheckResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Pre-publish checklist for %s@%s\n", m.Name, m.Version))
	b.WriteString(check(true, "Name", m.Name))
	b.WriteString(check(true, "Version", m.Version))
	b.WriteString(check(true, "Type", string(m.Type)))

	if m.Description != "" {
		desc := m.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		b.WriteString(check(true, "Description", desc))
	} else {
		b.WriteString(check(false, "Description", "missing"))
	}

	if m.CLI != nil {
		b.WriteString(check(m.CLI.Binary != "", "Binary", m.CLI.Binary))
		if m.CLI.Verify != "" {
			b.WriteString(check(true, "Verify", m.CLI.Verify))
		}
		if m.CLI.Auth != "" {
			auth := m.CLI.Auth
			if len(auth) > 50 {
				auth = auth[:47] + "..."
			}
			b.WriteString(check(true, "Auth hint", auth))
		}
	}

	if m.MCP != nil {
		b.WriteString(check(true, "Transport", m.MCP.Transport))
		if m.MCP.Command != "" {
			b.WriteString(check(true, "Command", m.MCP.Command))
		}
		if m.MCP.URL != "" {
			b.WriteString(check(true, "URL", m.MCP.URL))
		}
	}

	// Metadata fields
	if m.Author != "" {
		b.WriteString(check(true, "Author", m.Author))
	}
	if m.License != "" {
		b.WriteString(check(true, "License", m.License))
	}
	if m.Repository != "" {
		repo := m.Repository
		if len(repo) > 60 {
			repo = repo[:57] + "..."
		}
		b.WriteString(check(true, "Repository", repo))
	}

	// Install method results
	for _, r := range results {
		if r.OK {
			b.WriteString(check(true, "Install", fmt.Sprintf("%s: %s", r.Method, r.Pkg)))
		} else {
			b.WriteString(check(false, "Install", fmt.Sprintf("%s: %s (%s)", r.Method, r.Pkg, r.Error)))
		}
	}

	// Skill entry
	if m.Skill != nil && m.Skill.Entry != "" {
		b.WriteString(check(true, "Skill", m.Skill.Entry))
	}

	return b.String()
}

func check(ok bool, label, value string) string {
	marker := "[x]"
	if !ok {
		marker = "[!]"
	}
	return fmt.Sprintf("  %s %-14s %s\n", marker, label, value)
}
