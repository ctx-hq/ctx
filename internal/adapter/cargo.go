package adapter

import "context"

// CargoAdapter installs via cargo.
type CargoAdapter struct{}

func (a *CargoAdapter) Name() string      { return "cargo" }
func (a *CargoAdapter) Available() bool    { return commandExists("cargo") }

func (a *CargoAdapter) Install(ctx context.Context, pkg string) error {
	return runCommand(ctx, "cargo", "install", pkg)
}

func (a *CargoAdapter) Uninstall(ctx context.Context, pkg string) error {
	return runCommand(ctx, "cargo", "uninstall", pkg)
}
