package main

import (
	"fmt"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var unpublishCmd = &cobra.Command{
	Use:   "unpublish <package[@version]>",
	Short: "Permanently delete a package or version from the registry",
	Long: `Permanently remove a package or a specific version from getctx.org.

This is irreversible. All archives and metadata will be deleted.
If you only want to hide a version from resolution, use ctx yank instead.

Examples:
  ctx unpublish @hong/my-skill --yes          Delete entire package
  ctx unpublish @hong/my-skill@1.0.0 --yes    Delete a specific version`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireOnline(); err != nil {
			return err
		}
		w := getWriter(cmd)
		ref := args[0]

		// Parse: either @scope/name (whole package) or @scope/name@version
		fullName, version, isVersionDelete, err := parseUnpublishRef(ref)
		if err != nil {
			return output.ErrUsageHint(err.Error(), "Example: ctx unpublish @scope/name[@version]")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		if !flagYes {
			if isVersionDelete {
				return output.ErrUsageHint(
					fmt.Sprintf("this will permanently delete %s@%s", fullName, version),
					"Run with --yes to confirm",
				)
			}
			return output.ErrUsageHint(
				fmt.Sprintf("this will permanently delete %s and all its versions", fullName),
				"Run with --yes to confirm",
			)
		}

		reg := registry.New(cfg.RegistryURL(), token)

		if isVersionDelete {
			if err := reg.DeleteVersion(cmd.Context(), fullName, version); err != nil {
				return err
			}
			return w.OK(
				map[string]string{"deleted": "true", "full_name": fullName, "version": version},
				output.WithSummary("Deleted "+fullName+"@"+version),
			)
		}

		if err := reg.DeletePackage(cmd.Context(), fullName); err != nil {
			return err
		}
		return w.OK(
			map[string]string{"deleted": "true", "full_name": fullName},
			output.WithSummary("Deleted "+fullName),
		)
	},
}

// parseUnpublishRef parses a reference that may or may not include a version.
// For version deletes, it delegates to parsePackageRef for full validation.
// For whole-package deletes, it validates the @scope/name structure.
func parseUnpublishRef(ref string) (string, string, bool, error) {
	if !strings.HasPrefix(ref, "@") {
		return "", "", false, fmt.Errorf("invalid package reference: %s", ref)
	}

	// Check if there's a version component (second @)
	rest := ref[1:]
	atIdx := strings.LastIndex(rest, "@")

	if atIdx == -1 {
		// No version — whole package delete; validate @scope/name
		if err := validateScopedName(ref); err != nil {
			return "", "", false, err
		}
		return ref, "", false, nil
	}

	// Has version — delegate to parsePackageRef for full validation
	fullName, version, err := parsePackageRef(ref)
	if err != nil {
		return "", "", false, err
	}
	return fullName, version, true, nil
}

// validateScopedName checks that ref is a valid @scope/name.
func validateScopedName(ref string) error {
	if !strings.HasPrefix(ref, "@") {
		return fmt.Errorf("invalid package reference: expected @scope/name, got %s", ref)
	}
	slashIdx := strings.Index(ref[1:], "/")
	if slashIdx == -1 || slashIdx == 0 || slashIdx == len(ref[1:])-1 {
		return fmt.Errorf("invalid package reference: expected @scope/name, got %s", ref)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(unpublishCmd)
}
