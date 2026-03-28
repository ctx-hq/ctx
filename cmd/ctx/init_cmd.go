package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
	"github.com/spf13/cobra"
)

var initType string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ctx.yaml in the current directory",
	Long: `Scaffold a new ctx.yaml manifest for a skill, MCP server, or CLI tool.

Examples:
  ctx init --type skill
  ctx init --type mcp
  ctx init --type cli`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// Check if ctx.yaml already exists
		if _, err := os.Stat(manifest.FileName); err == nil {
			return output.ErrUsage(manifest.FileName + " already exists in this directory")
		}

		pkgType := manifest.PackageType(initType)
		if !pkgType.Valid() {
			return output.ErrUsageHint(
				fmt.Sprintf("invalid type %q", initType),
				"Must be skill, mcp, or cli",
			)
		}

		// Derive name from current directory
		cwd, _ := os.Getwd()
		dirName := filepath.Base(cwd)
		scope := "your-scope" // placeholder

		m := manifest.Scaffold(pkgType, scope, dirName)
		data, err := manifest.Marshal(m)
		if err != nil {
			return err
		}

		if err := os.WriteFile(manifest.FileName, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", manifest.FileName, err)
		}

		return w.OK(
			map[string]string{"file": manifest.FileName, "type": initType},
			output.WithSummary("Created "+manifest.FileName+" (type: "+initType+")"),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "validate", Command: "ctx val", Description: "Validate the manifest"},
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish to registry"},
			),
		)
	},
}

func init() {
	initCmd.Flags().StringVarP(&initType, "type", "t", "skill", "Package type (skill, mcp, cli)")
}
