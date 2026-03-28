package main

import (
	"fmt"

	"github.com/getctx/ctx/internal/output"
	"github.com/getctx/ctx/internal/selfupdate"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade ctx to the latest version",
	Long:  "Download and install the latest version of ctx.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		output.Info("Checking for updates...")
		latest := selfupdate.FetchLatestVersion()
		if latest == "" {
			return fmt.Errorf("could not determine latest version")
		}

		current := Version
		if current == latest || current == "v"+latest {
			output.Success("ctx %s is already the latest version", current)
			return w.OK(map[string]string{
				"version": current,
				"status":  "up_to_date",
			}, output.WithSummary(fmt.Sprintf("ctx %s is already the latest version", current)))
		}

		output.Info("Upgrading ctx %s → %s...", current, latest)

		if err := selfupdate.Upgrade(latest); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}

		output.Success("ctx upgraded to %s", latest)

		return w.OK(map[string]string{
			"previous": current,
			"current":  latest,
		}, output.WithSummary(fmt.Sprintf("Upgraded ctx %s → %s", current, latest)))
	},
}
