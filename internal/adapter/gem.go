package adapter

import "context"

// GemAdapter installs via Ruby Gems.
type GemAdapter struct{}

func (a *GemAdapter) Name() string   { return "gem" }
func (a *GemAdapter) Available() bool { return commandExists("gem") }

func (a *GemAdapter) Install(ctx context.Context, pkg string) error {
	return runCommand(ctx, "gem", "install", pkg)
}

func (a *GemAdapter) Uninstall(ctx context.Context, pkg string) error {
	return runCommand(ctx, "gem", "uninstall", pkg, "-a", "-x")
}
