package adapter

import "context"

// BrewAdapter installs via Homebrew.
type BrewAdapter struct{}

func (a *BrewAdapter) Name() string      { return "brew" }
func (a *BrewAdapter) Available() bool    { return commandExists("brew") }

func (a *BrewAdapter) Install(ctx context.Context, pkg string) error {
	return runCommand(ctx, "brew", "install", pkg)
}

func (a *BrewAdapter) Uninstall(ctx context.Context, pkg string) error {
	return runCommand(ctx, "brew", "uninstall", pkg)
}
