package main

import (
	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"dr"},
	Short:   "Diagnose environment and connectivity",
	Long: `Run diagnostic checks to verify your ctx installation,
configuration, network connectivity, and detected agents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		result := doctor.RunChecks(Version, getToken())

		return w.OK(result.Checks,
			output.WithSummary(result.Summary()),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "link", Command: "ctx ln", Description: "Link packages to agents"},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
