package main

import (
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:     "validate [path]",
	Aliases: []string{"val"},
	Short:   "Validate a ctx.yaml file",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		m, err := manifest.LoadFromDir(dir)
		if err != nil {
			return output.ErrUsage("load manifest: " + err.Error())
		}

		errs := manifest.Validate(m)

		// If this is a skill package, also validate SKILL.md if present
		var skillWarnings []string
		if m.Type == manifest.TypeSkill || (m.Skill != nil && m.Skill.Entry != "") {
			entry := "SKILL.md"
			if m.Skill != nil && m.Skill.Entry != "" {
				entry = m.Skill.Entry
			}
			skillPath := filepath.Join(dir, entry)
			if f, openErr := os.Open(skillPath); openErr == nil {
				defer f.Close()
				fm, _, parseErr := manifest.ParseSkillMD(f)
				if parseErr != nil {
					skillWarnings = append(skillWarnings, "SKILL.md frontmatter parse error: "+parseErr.Error())
				} else {
					skillWarnings = manifest.ValidateSkillMD(fm, m)
				}
			}
			// If SKILL.md doesn't exist, that's not an error — it might be created later
		}

		result := map[string]any{
			"valid":          len(errs) == 0,
			"errors":         errs,
			"skill_warnings": skillWarnings,
			"name":           m.Name,
			"type":           m.Type,
		}

		if len(errs) > 0 {
			_ = w.OK(result, output.WithSummary("validation failed"))
			return output.ErrUsageHint(
				errs[0],
				"Fix the errors above and run 'ctx validate' again",
			)
		}

		notice := ""
		if len(skillWarnings) > 0 {
			notice = "SKILL.md warnings:"
			for _, sw := range skillWarnings {
				notice += "\n  - " + sw
			}
		}

		opts := []output.ResponseOption{
			output.WithSummary("Valid: " + m.Name + " (" + string(m.Type) + ") v" + m.Version),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish to registry"},
			),
		}
		if notice != "" {
			opts = append(opts, output.WithNotice(notice))
		}

		return w.OK(result, opts...)
	},
}
