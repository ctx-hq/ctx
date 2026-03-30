package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <package> <new-name>",
	Short: "Rename a package (within the same scope)",
	Long: `Rename a package. The old name will redirect to the new one.

Examples:
  ctx rename @alice/old-name new-name --yes`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if !flagYes {
			return output.ErrUsageHint(
				fmt.Sprintf("this will rename package %s to %s", args[0], args[1]),
				"Run with --yes to confirm",
			)
		}

		result, err := reg.RenamePackage(cmd.Context(), args[0], args[1], args[0])
		if err != nil {
			return err
		}
		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Renamed: %s → %s", args[0], args[1])),
		)
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
