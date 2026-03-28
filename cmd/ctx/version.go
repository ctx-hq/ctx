package main

import (
	"runtime"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print ctx version",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		return w.OK(map[string]string{
			"version": Version,
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		}, output.WithSummary("ctx "+Version))
	},
}
