package config

import (
	"fmt"
	"os"
	"runtime"
)

const (
	// DirPerm is the permission for directories created by ctx.
	// Owner-only access to protect sensitive data.
	DirPerm os.FileMode = 0o700

	// FilePerm is the permission for files that may contain sensitive data
	// (tokens, config, link registry, caches).
	FilePerm os.FileMode = 0o600

	// BinPerm is the permission for executable binaries.
	BinPerm os.FileMode = 0o755
)

// Version is set by the main package at startup.
var Version string

// UserAgent returns the standard User-Agent string for HTTP requests.
func UserAgent() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("ctx/%s (%s/%s)", v, runtime.GOOS, runtime.GOARCH)
}
