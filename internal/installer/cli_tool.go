package installer

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ctx-hq/ctx/internal/adapter"
	"github.com/ctx-hq/ctx/internal/installstate"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
)

// InstallCLI installs a CLI tool using the best available adapter.
// Returns the CLI installation state for tracking.
func InstallCLI(ctx context.Context, m *manifest.Manifest) (*installstate.CLIState, error) {
	if m.CLI == nil {
		return nil, fmt.Errorf("package is not a CLI tool")
	}
	if m.Install == nil {
		return nil, fmt.Errorf("no install spec for CLI tool %s", m.Name)
	}

	spec := adapter.InstallSpec{
		Source: m.Install.Source,
		Brew:   m.Install.Brew,
		Npm:    m.Install.Npm,
		Pip:    m.Install.Pip,
		Gem:    m.Install.Gem,
		Cargo:  m.Install.Cargo,
		Script: m.Install.Script,
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
		return nil, err
	}

	state := &installstate.CLIState{
		Adapter:    a.Name(),
		AdapterPkg: pkg,
		Binary:     m.CLI.Binary,
		Status:     "failed",
	}

	output.FromContext(ctx).Info("Installing via %s: %s", a.Name(), pkg)
	if err := a.Install(ctx, pkg); err != nil {
		return state, fmt.Errorf("install via %s: %w", a.Name(), err)
	}

	// Verify installation
	if err := adapter.Verify(m.CLI.Binary, m.CLI.Verify); err != nil {
		return state, fmt.Errorf("installed but verify failed: %w", err)
	}

	// Resolve binary path
	if binaryPath, lookErr := exec.LookPath(m.CLI.Binary); lookErr == nil {
		state.BinaryPath = binaryPath
	}
	state.Verified = true
	state.Status = "ok"

	output.FromContext(ctx).Success("Verified: %s is available", m.CLI.Binary)

	// Register in links registry
	links, linkErr := LoadLinks()
	if linkErr != nil {
		links = &LinkRegistry{Version: linksFileVersion, Links: make(map[string][]LinkEntry)}
	}
	links.Add(m.Name, LinkEntry{
		Agent:  a.Name(),
		Type:   LinkBinary,
		Source: pkg,
		Target: m.CLI.Binary,
	})
	_ = links.Save() // best effort

	return state, nil
}
