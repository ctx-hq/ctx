package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var (
	artifactPlatform string
	artifactFile     string
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Manage platform-specific binary artifacts",
	Long: `Upload and list platform-specific binary artifacts for package versions.

Examples:
  ctx artifact upload @scope/pkg@1.0.0 --platform darwin-arm64 --file dist/bin.tar.gz
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

		if artifactPlatform == "" {
			return fmt.Errorf("--platform is required")
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

	artifactCmd.AddCommand(artifactUploadCmd)
	artifactCmd.AddCommand(artifactListCmd)
	rootCmd.AddCommand(artifactCmd)
}
