package main

import (
	"os"
	"runtime/debug"

	"github.com/ctx-hq/ctx/internal/output"
)

func init() {
	resolveVersionFromBuildInfo()
}

// resolveVersionFromBuildInfo sets Version from Go module build info
// when it has not been injected via ldflags (i.e., still "dev").
// This enables `go install github.com/ctx-hq/ctx/cmd/ctx@latest`
// to report the correct module version.
func resolveVersionFromBuildInfo() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Resolve format from flags directly — the writer in rootCmd's context
		// may not have the user-specified format because PersistentPreRunE sets
		// it on the subcommand's context, not rootCmd's.
		format, _ := output.ResolveFormat(flagJSON, flagQuiet, flagMD, flagIDsOnly, flagCount, flagAgent)
		colorMode, _ := output.ParseColorMode(flagColor)
		w := output.NewWriter(output.WithFormat(format), output.WithColorMode(colorMode))
		_ = w.Err(err)

		if cliErr := output.AsCLIError(err); cliErr != nil {
			os.Exit(cliErr.ExitCode())
		}
		os.Exit(1)
	}
}
