package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/introspect"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var flagWrapOutputDir string

var wrapCmd = &cobra.Command{
	Use:   "wrap <binary>",
	Short: "Package a CLI tool as a ctx skill",
	Long: `Introspect an installed CLI binary and generate ctx.yaml + SKILL.md
for publishing to getctx.org.

This wraps a pure CLI tool (like ffmpeg, jq, ripgrep) with agent skill
metadata so it can be discovered and used by AI agents.

Examples:
  ctx wrap ffmpeg              Wrap ffmpeg in current directory
  ctx wrap jq -o ./jq-pkg     Write files to a specific directory
  ctx wrap ollama -y           Skip prompts, use defaults`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		binary := args[0]

		// Step 1: Verify binary exists
		binPath, err := exec.LookPath(binary)
		if err != nil {
			return fmt.Errorf("binary %q not found in PATH", binary)
		}
		w.Info("Found %s at %s", binary, binPath)

		// Step 2: Capture help text
		helpText, helpErr := introspect.CaptureHelp(binary)
		if helpErr != nil {
			w.Warn("Could not capture help output: %v", helpErr)
		}

		// Step 3: Detect version
		vr := introspect.CaptureVersion(binary)
		version := vr.Version
		w.PrintDim("  Detected version: %s", version)

		// Step 4: Detect install method
		installSpec := introspect.DetectInstallMethod(binary)
		if installSpec.Brew != "" {
			w.PrintDim("  Install method: brew (%s)", installSpec.Brew)
		} else if installSpec.Npm != "" {
			w.PrintDim("  Install method: npm (%s)", installSpec.Npm)
		} else if installSpec.Pip != "" {
			w.PrintDim("  Install method: pip (%s)", installSpec.Pip)
		} else if installSpec.Cargo != "" {
			w.PrintDim("  Install method: cargo (%s)", installSpec.Cargo)
		} else {
			w.Warn("No install method detected — users won't be able to install this package.")
			w.PrintDim("  Edit ctx.yaml to add install.brew, install.npm, install.pip, install.cargo, or install.script before publishing.")
		}

		// Step 5: Determine metadata
		scope := resolvedUsername()
		if scope == "" {
			scope = "community"
		}
		name := binary

		description := ""
		if helpText != "" {
			// Use first non-empty line as description
			for _, line := range strings.Split(helpText, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "Usage") && !strings.HasPrefix(trimmed, "usage") {
					description = trimmed
					break
				}
			}
		}
		if description == "" {
			description = fmt.Sprintf("CLI tool: %s", binary)
		}
		// Truncate long descriptions
		if len(description) > 200 {
			description = description[:200]
		}

		if !flagYes {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "Package name [@%s/%s]: ", scope, name)
			var input string
			if _, err := fmt.Scanln(&input); err == nil && input != "" {
				name = input
			}

			fmt.Fprintf(os.Stderr, "Description [%s]: ", truncate(description, 60))
			input = ""
			if _, err := fmt.Scanln(&input); err == nil && input != "" {
				description = input
			}
		}

		// Step 6: Build manifest
		m := &manifest.Manifest{
			Name:        manifest.FormatFullName(scope, name),
			Version:     version,
			Type:        manifest.TypeCLI,
			Description: description,
			CLI: &manifest.CLISpec{
				Binary: binary,
				Verify: fmt.Sprintf("%s %s", binary, vr.Command),
			},
			Skill: &manifest.SkillSpec{
				Entry:  "SKILL.md",
				Origin: "wrapped",
			},
			Install: installSpec,
		}

		errs := manifest.Validate(m)
		if len(errs) > 0 {
			for _, e := range errs {
				w.Warn("Validation: %s", e)
			}
			return fmt.Errorf("generated manifest has validation errors")
		}

		// Step 7: Generate SKILL.md
		skillContent := introspect.GenerateSkillMD(binary, description, helpText)

		// Step 8: Write files
		outDir := flagWrapOutputDir
		if outDir == "" {
			outDir = "."
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}

		manifestData, err := manifest.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal manifest: %w", err)
		}

		ctxYamlPath := filepath.Join(outDir, "ctx.yaml")
		skillPath := filepath.Join(outDir, "SKILL.md")

		if err := os.WriteFile(ctxYamlPath, manifestData, 0o644); err != nil {
			return fmt.Errorf("write ctx.yaml: %w", err)
		}
		if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
			return fmt.Errorf("write SKILL.md: %w", err)
		}

		return w.OK(map[string]string{
			"ctx_yaml": ctxYamlPath,
			"skill_md": skillPath,
			"package":  m.Name,
			"version":  m.Version,
		},
			output.WithSummary(fmt.Sprintf("Created ctx.yaml + SKILL.md for %s", binary)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "edit", Command: fmt.Sprintf("$EDITOR %s", skillPath), Description: "Improve the generated skill"},
				output.Breadcrumb{Action: "publish", Command: fmt.Sprintf("ctx publish %s", outDir), Description: "Publish to getctx.org"},
			),
		)
	},
}

func init() {
	wrapCmd.Flags().StringVarP(&flagWrapOutputDir, "output-dir", "o", "", "Directory to write ctx.yaml and SKILL.md (default: current directory)")
	rootCmd.AddCommand(wrapCmd)
}

