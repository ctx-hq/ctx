package selfupdate

import (
	"testing"
)

func TestFetchLatestVersionExported(t *testing.T) {
	// FetchLatestVersion is the exported wrapper around fetchLatestVersion.
	// We just verify it doesn't panic. In CI or offline environments it may
	// return "" which is fine.
	_ = FetchLatestVersion()
}
