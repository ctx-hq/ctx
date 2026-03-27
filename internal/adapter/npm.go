package adapter

import "context"

// NpmAdapter installs via npm global.
type NpmAdapter struct{}

func (a *NpmAdapter) Name() string      { return "npm" }
func (a *NpmAdapter) Available() bool    { return commandExists("npm") }

func (a *NpmAdapter) Install(ctx context.Context, pkg string) error {
	return runCommand(ctx, "npm", "install", "-g", pkg)
}

func (a *NpmAdapter) Uninstall(ctx context.Context, pkg string) error {
	return runCommand(ctx, "npm", "uninstall", "-g", pkg)
}
