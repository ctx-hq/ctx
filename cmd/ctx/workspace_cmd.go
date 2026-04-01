package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/gitutil"
	"github.com/ctx-hq/ctx/internal/license"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
	Short:   "Manage multi-skill workspaces (monorepos)",
	Long: `Manage workspaces containing multiple skills in a single repository.

Examples:
  ctx workspace init --scan "skills/*" --scope "@myname"
  ctx workspace list
  ctx workspace validate`,
}

// --- workspace list ---

var workspaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List workspace members",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		ws, err := manifest.LoadWorkspace(".")
		if err != nil {
			return err
		}

		type memberRow struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  string `json:"source"`
			RelDir  string `json:"dir"`
		}

		rows := make([]memberRow, 0, len(ws.Members))
		for _, m := range ws.Members {
			rows = append(rows, memberRow{
				Name:    m.Manifest.Name,
				Version: m.Manifest.Version,
				Source:  m.Source,
				RelDir:  m.RelDir,
			})
		}

		return w.OK(rows,
			output.WithSummary(fmt.Sprintf("%d members in workspace %s", len(rows), ws.Root.Name)),
		)
	},
}

// --- workspace init ---

var (
	flagWsScan    string
	flagWsExclude string
	flagWsScope   string
)

var workspaceInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a workspace from an existing multi-skill repo",
	Long: `Scan for skills and generate ctx.yaml files.

If .claude-plugin/marketplace.json exists, collections are auto-detected.

Examples:
  ctx workspace init --scan "skills/*" --scope "@myname"
  ctx workspace init --scan "*" --exclude "docs,scripts" --scope "@team"
  ctx workspace init --scan "marketing/*,engineering/*" --scope "@org"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootDir := "."

		// Check if workspace already exists.
		if _, err := os.Stat(filepath.Join(rootDir, manifest.FileName)); err == nil {
			return output.ErrUsageHint(
				"ctx.yaml already exists in this directory",
				"Delete it first or run 'ctx workspace list' to see members",
			)
		}

		// Parse scan patterns.
		scanPatterns := strings.Split(flagWsScan, ",")
		for i := range scanPatterns {
			scanPatterns[i] = strings.TrimSpace(scanPatterns[i])
		}

		// Parse exclude patterns.
		var excludePatterns []string
		if flagWsExclude != "" {
			for _, e := range strings.Split(flagWsExclude, ",") {
				e = strings.TrimSpace(e)
				if e != "" {
					excludePatterns = append(excludePatterns, e)
				}
			}
		}

		// Resolve member directories.
		absRoot, err := filepath.Abs(rootDir)
		if err != nil {
			return fmt.Errorf("resolve workspace root: %w", err)
		}
		dirs, err := manifest.ResolveMembers(absRoot, scanPatterns, excludePatterns)
		if err != nil {
			return err
		}

		if len(dirs) == 0 {
			return output.ErrUsageHint(
				"no skills found matching patterns: "+flagWsScan,
				"Ensure directories contain a SKILL.md file",
			)
		}

		// Scaffold ctx.yaml for each skill dir that doesn't already have one.
		created := 0
		skipped := 0
		for _, dir := range dirs {
			ctxPath := filepath.Join(dir, manifest.FileName)
			if fileExists(ctxPath) {
				skipped++
				continue
			}

			m, scaffoldErr := manifest.ScaffoldFromSkillMD(dir)
			if scaffoldErr != nil {
				relDir, _ := filepath.Rel(absRoot, dir)
				output.Warn("Skipping %s: %v", relDir, scaffoldErr)
				skipped++
				continue
			}

			// Apply scope if provided.
			if flagWsScope != "" {
				manifest.ApplyDefaults(m, &manifest.WorkspaceDefaults{Scope: flagWsScope})
			}

			// Auto-enrich: fill author, repository, license from git/filesystem.
			// Uses workspace root for git info (individual skill dirs rarely have their own .git).
			if m.Author == "" {
				m.Author = gitutil.Author(absRoot)
			}
			if m.Repository == "" {
				m.Repository = gitutil.RemoteURL(absRoot)
			}
			if m.License == "" {
				if lr := license.Detect(absRoot); lr.SPDX != "" {
					m.License = lr.SPDX
				}
			}

			data, marshalErr := manifest.Marshal(m)
			if marshalErr != nil {
				return marshalErr
			}

			if writeErr := os.WriteFile(ctxPath, data, 0o644); writeErr != nil {
				return fmt.Errorf("write %s: %w", ctxPath, writeErr)
			}
			created++
			relDir, _ := filepath.Rel(absRoot, dir)
			output.PrintDim("  Created %s/%s", relDir, manifest.FileName)
		}

		// Auto-detect collections from .claude-plugin/marketplace.json.
		collections := detectMarketplaceCollections(rootDir)

		// Generate root ctx.yaml.
		rootName := "skills"
		if flagWsScope != "" {
			rootName = manifest.FormatFullName(strings.TrimPrefix(flagWsScope, "@"), "skills")
		}
		rootManifest := &manifest.Manifest{
			Name:        rootName,
			Type:        manifest.TypeWorkspace,
			Description: "Skill workspace",
			Workspace: &manifest.WorkspaceSpec{
				Members: scanPatterns,
				Exclude: excludePatterns,
			},
		}

		// Populate workspace defaults from git/filesystem.
		defaults := &manifest.WorkspaceDefaults{}
		if flagWsScope != "" {
			defaults.Scope = flagWsScope
		}
		if author := gitutil.Author(absRoot); author != "" {
			defaults.Author = author
		}
		if repo := gitutil.RemoteURL(absRoot); repo != "" {
			defaults.Repository = repo
		}
		if lr := license.Detect(absRoot); lr.SPDX != "" {
			defaults.License = lr.SPDX
		}
		if defaults.Scope != "" || defaults.Author != "" || defaults.Repository != "" || defaults.License != "" {
			rootManifest.Workspace.Defaults = defaults
		}

		if len(collections) > 0 {
			rootManifest.Workspace.Collections = collections
		}

		rootData, err := manifest.Marshal(rootManifest)
		if err != nil {
			return err
		}

		if writeErr := os.WriteFile(filepath.Join(rootDir, manifest.FileName), rootData, 0o644); writeErr != nil {
			return fmt.Errorf("write root %s: %w", manifest.FileName, writeErr)
		}

		output.Info("Workspace initialized: %d skills created, %d skipped", created, skipped)
		output.Info("Root %s created with %d member pattern(s)", manifest.FileName, len(scanPatterns))
		if len(collections) > 0 {
			output.Info("Auto-detected %d collection(s) from .claude-plugin/marketplace.json", len(collections))
		}

		return nil
	},
}

// --- workspace validate ---

var workspaceValidateCmd = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"val"},
	Short:   "Validate all workspace members",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := manifest.LoadWorkspace(".")
		if err != nil {
			return err
		}

		totalErrors := 0
		for _, m := range ws.Members {
			errs := manifest.Validate(m.Manifest)
			if len(errs) > 0 {
				totalErrors += len(errs)
				for _, e := range errs {
					output.Warn("%s: %s", m.RelDir, e)
				}
			}
		}

		// Validate collections if present.
		if ws.Root.Workspace != nil && len(ws.Root.Workspace.Collections) > 0 {
			_, colErr := manifest.ResolveCollections(ws)
			if colErr != nil {
				totalErrors++
				output.Warn("collections: %v", colErr)
			}
		}

		if totalErrors > 0 {
			return fmt.Errorf("%d validation error(s) found", totalErrors)
		}

		output.Info("All %d workspace members are valid", len(ws.Members))
		return nil
	},
}

// --- init registration ---

func init() {
	workspaceInitCmd.Flags().StringVar(&flagWsScan, "scan", "skills/*", "Glob pattern(s) to scan for skills (comma-separated)")
	workspaceInitCmd.Flags().StringVar(&flagWsExclude, "exclude", "", "Directory names to exclude (comma-separated)")
	workspaceInitCmd.Flags().StringVar(&flagWsScope, "scope", "", "Default scope for skill names (e.g., @myname)")

	workspaceCmd.AddCommand(workspaceListCmd, workspaceInitCmd, workspaceValidateCmd)
}

// --- helpers ---

type marketplaceJSON struct {
	Plugins []marketplacePlugin `json:"plugins"`
}

type marketplacePlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
}

func detectMarketplaceCollections(rootDir string) []manifest.CollectionSpec {
	path := filepath.Join(rootDir, ".claude-plugin", "marketplace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var mp marketplaceJSON
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil
	}

	var collections []manifest.CollectionSpec
	for _, plugin := range mp.Plugins {
		if len(plugin.Skills) == 0 {
			continue
		}

		// Extract short names from skill paths like "./skills/xlsx"
		var members []string
		for _, s := range plugin.Skills {
			s = strings.TrimPrefix(s, "./")
			name := filepath.Base(s)
			members = append(members, name)
		}

		collections = append(collections, manifest.CollectionSpec{
			Name:        plugin.Name,
			Description: plugin.Description,
			Members:     members,
		})
	}

	return collections
}

// fileExists checks if a file exists (used by init command).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

