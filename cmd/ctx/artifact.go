package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var (
	artifactPlatform string
	artifactFile     string
	artifactDir      string
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Manage platform-specific binary artifacts",
	Long: `Upload and list platform-specific binary artifacts for package versions.

Examples:
  ctx artifact upload @scope/pkg@1.0.0 --platform darwin-arm64 --file dist/bin.tar.gz
  ctx artifact upload @scope/pkg@1.0.0 --dir dist/
  ctx artifact list @scope/pkg@1.0.0`,
}

var artifactUploadCmd = &cobra.Command{
	Use:   "upload <package>@<version>",
	Short: "Upload a platform-specific artifact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, client, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		fullName, version, err := parsePackageVersion(args[0])
		if err != nil {
			return err
		}

		// --dir mode: auto-detect and upload all platform archives
		if artifactDir != "" {
			return uploadFromDir(cmd, w, client, fullName, version, artifactDir)
		}

		if artifactPlatform == "" {
			return fmt.Errorf("--platform is required (or use --dir to auto-detect)")
		}
		if artifactFile == "" {
			return fmt.Errorf("--file is required")
		}

		f, err := os.Open(artifactFile)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer func() { _ = f.Close() }()

		if err := client.UploadArtifact(cmd.Context(), fullName, version, artifactPlatform, f); err != nil {
			return err
		}

		return w.OK(
			map[string]string{
				"package":  fullName,
				"version":  version,
				"platform": artifactPlatform,
				"status":   "uploaded",
			},
			output.WithSummary(fmt.Sprintf("Uploaded artifact for %s@%s (%s)", fullName, version, artifactPlatform)),
		)
	},
}

// platformFromFilename tries to extract a platform string (e.g., "darwin-arm64")
// from a goreleaser-style archive filename like "myapp_0.2.0_darwin_arm64.tar.gz".
func platformFromFilename(name string) string {
	// Strip .tar.gz or .zip extension
	base := strings.TrimSuffix(name, ".tar.gz")
	base = strings.TrimSuffix(base, ".zip")

	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return ""
	}

	// Last two segments should be os_arch
	osName := parts[len(parts)-2]
	arch := parts[len(parts)-1]

	validOS := map[string]bool{"darwin": true, "linux": true, "windows": true}
	validArch := map[string]bool{"amd64": true, "arm64": true, "386": true}

	if validOS[osName] && validArch[arch] {
		return osName + "-" + arch
	}
	return ""
}

// goreleaserArtifact represents one entry from goreleaser's artifacts.json.
type goreleaserArtifact struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Goos  string `json:"goos"`
	Goarch string `json:"goarch"`
	Type  string `json:"type"`
}

// uploadFromDir scans a directory for platform archives and uploads them.
// It tries goreleaser artifacts.json first, then falls back to filename detection.
func uploadFromDir(cmd *cobra.Command, w *output.Writer, client *registry.Client, fullName, version, dir string) error {
	type uploadItem struct {
		platform string
		path     string
	}
	var items []uploadItem

	// Try goreleaser artifacts.json first
	artifactsJSON := filepath.Join(dir, "artifacts.json")
	if data, err := os.ReadFile(artifactsJSON); err == nil {
		var artifacts []goreleaserArtifact
		if json.Unmarshal(data, &artifacts) == nil {
			for _, a := range artifacts {
				if a.Type != "Archive" || a.Goos == "" || a.Goarch == "" {
					continue
				}
				platform := a.Goos + "-" + a.Goarch
				path := a.Path
				if !filepath.IsAbs(path) {
					path = filepath.Join(dir, path)
				}
				items = append(items, uploadItem{platform: platform, path: path})
			}
		}
	}

	// Fallback: scan directory for *.tar.gz files with platform in name
	if len(items) == 0 {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
				continue
			}
			platform := platformFromFilename(name)
			if platform != "" {
				items = append(items, uploadItem{platform: platform, path: filepath.Join(dir, name)})
			}
		}
	}

	if len(items) == 0 {
		return output.ErrUsageHint(
			"no platform archives found in "+dir,
			"Expected goreleaser artifacts.json or files named like myapp_0.2.0_darwin_arm64.tar.gz",
		)
	}

	var uploaded int
	var results []map[string]string
	for _, item := range items {
		f, err := os.Open(item.path)
		if err != nil {
			output.Warn("skip %s: %v", item.path, err)
			continue
		}

		output.Info("Uploading %s (%s)...", filepath.Base(item.path), item.platform)
		if err := client.UploadArtifact(cmd.Context(), fullName, version, item.platform, f); err != nil {
			_ = f.Close()
			output.Warn("failed %s: %v", item.platform, err)
			continue
		}
		_ = f.Close()
		uploaded++
		results = append(results, map[string]string{"platform": item.platform, "file": filepath.Base(item.path)})
	}

	if uploaded == 0 {
		return fmt.Errorf("all %d artifact uploads failed", len(items))
	}

	return w.OK(
		map[string]interface{}{
			"package":   fullName,
			"version":   version,
			"uploaded":  uploaded,
			"total":     len(items),
			"artifacts": results,
		},
		output.WithSummary(fmt.Sprintf("Uploaded %d/%d artifacts for %s@%s", uploaded, len(items), fullName, version)),
	)
}

var artifactListCmd = &cobra.Command{
	Use:     "list <package>@<version>",
	Short:   "List artifacts for a version",
	Aliases: []string{"ls"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, client, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		fullName, version, err := parsePackageVersion(args[0])
		if err != nil {
			return err
		}

		artifacts, err := client.ListArtifacts(cmd.Context(), fullName, version)
		if err != nil {
			return err
		}

		return w.OK(artifacts,
			output.WithSummary(fmt.Sprintf("%d artifact(s) for %s@%s", len(artifacts), fullName, version)),
		)
	},
}

// parsePackageVersion splits "@scope/name@version" into fullName and version.
func parsePackageVersion(ref string) (string, string, error) {
	// Handle @scope/name@version
	if strings.HasPrefix(ref, "@") {
		rest := ref[1:]
		slashIdx := strings.Index(rest, "/")
		if slashIdx == -1 {
			return "", "", fmt.Errorf("invalid package reference %q: expected @scope/name@version", ref)
		}
		atIdx := strings.Index(rest[slashIdx:], "@")
		if atIdx == -1 {
			return "", "", fmt.Errorf("invalid package reference %q: missing @version", ref)
		}
		atIdx += slashIdx
		fullName := ref[:atIdx+1]
		version := rest[atIdx+1:]
		// Validate: scope and name segments must be non-empty
		if slashIdx == 0 || atIdx == slashIdx+1 || version == "" {
			return "", "", fmt.Errorf("invalid package reference %q: expected @scope/name@version", ref)
		}
		return fullName, version, nil
	}

	// Handle name@version without @scope
	atIdx := strings.LastIndex(ref, "@")
	if atIdx <= 0 || atIdx == len(ref)-1 {
		return "", "", fmt.Errorf("invalid package reference %q: missing @version", ref)
	}
	return ref[:atIdx], ref[atIdx+1:], nil
}

func init() {
	artifactUploadCmd.Flags().StringVar(&artifactPlatform, "platform", "", "Target platform (e.g. darwin-arm64, linux-amd64)")
	artifactUploadCmd.Flags().StringVar(&artifactFile, "file", "", "Path to artifact archive")
	artifactUploadCmd.Flags().StringVar(&artifactDir, "dir", "", "Directory with platform archives (auto-detect from goreleaser)")

	artifactCmd.AddCommand(artifactUploadCmd)
	artifactCmd.AddCommand(artifactListCmd)
	rootCmd.AddCommand(artifactCmd)
}
