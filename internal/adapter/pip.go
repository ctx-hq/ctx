package adapter

import "context"

// PipAdapter installs via pip/pipx.
type PipAdapter struct{}

func (a *PipAdapter) Name() string { return "pip" }

func (a *PipAdapter) Available() bool {
	return commandExists("pipx") || commandExists("pip3")
}

func (a *PipAdapter) Install(ctx context.Context, pkg string) error {
	if commandExists("pipx") {
		return runCommand(ctx, "pipx", "install", pkg)
	}
	return runCommand(ctx, "pip3", "install", pkg)
}

func (a *PipAdapter) Uninstall(ctx context.Context, pkg string) error {
	if commandExists("pipx") {
		return runCommand(ctx, "pipx", "uninstall", pkg)
	}
	return runCommand(ctx, "pip3", "uninstall", "-y", pkg)
}
