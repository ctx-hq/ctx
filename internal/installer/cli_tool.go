package installer

import (
	"context"
	"fmt"

	"github.com/getctx/ctx/internal/adapter"
	"github.com/getctx/ctx/internal/manifest"
	"github.com/getctx/ctx/internal/output"
)

// InstallCLI installs a CLI tool using the best available adapter.
func InstallCLI(ctx context.Context, m *manifest.Manifest) error {
	if m.CLI == nil {
		return fmt.Errorf("package is not a CLI tool")
	}
	if m.Install == nil {
		return fmt.Errorf("no install spec for CLI tool %s", m.Name)
	}

	spec := adapter.InstallSpec{
		Source: m.Install.Source,
		Brew:   m.Install.Brew,
		Npm:    m.Install.Npm,
		Pip:    m.Install.Pip,
		Cargo:  m.Install.Cargo,
	}

	if m.Install.Platforms != nil {
		spec.Platforms = make(map[string]adapter.PlatformSpec)
		for k, v := range m.Install.Platforms {
			spec.Platforms[k] = adapter.PlatformSpec{
				Source: v.Source,
				Brew:   v.Brew,
				Npm:    v.Npm,
				Binary: v.Binary,
			}
		}
	}

	a, pkg, err := adapter.FindAdapter(spec)
	if err != nil {
		return err
	}

	output.Info("Installing via %s: %s", a.Name(), pkg)
	if err := a.Install(ctx, pkg); err != nil {
		return fmt.Errorf("install via %s: %w", a.Name(), err)
	}

	// Verify installation
	if err := adapter.Verify(m.CLI.Binary, m.CLI.Verify); err != nil {
		return fmt.Errorf("installed but verify failed: %w", err)
	}

	output.Success("Verified: %s is available", m.CLI.Binary)
	return nil
}
