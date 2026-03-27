package adapter

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Adapter installs a CLI tool via a specific package manager.
type Adapter interface {
	Name() string
	Available() bool
	Install(ctx context.Context, pkg string) error
	Uninstall(ctx context.Context, pkg string) error
}

// Platform returns the current platform string like "darwin-arm64".
func Platform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// DetectAdapters returns all available adapters on the current system.
func DetectAdapters() []Adapter {
	all := []Adapter{
		&BrewAdapter{},
		&NpmAdapter{},
		&PipAdapter{},
		&CargoAdapter{},
		&BinaryAdapter{},
	}
	var available []Adapter
	for _, a := range all {
		if a.Available() {
			available = append(available, a)
		}
	}
	return available
}

// FindAdapter returns the best adapter for a given install spec.
func FindAdapter(spec InstallSpec) (Adapter, string, error) {
	platform := Platform()

	// Check platform-specific override first
	if spec.Platforms != nil {
		if pSpec, ok := spec.Platforms[platform]; ok {
			if pSpec.Brew != "" && commandExists("brew") {
				return &BrewAdapter{}, pSpec.Brew, nil
			}
			if pSpec.Npm != "" && commandExists("npm") {
				return &NpmAdapter{}, pSpec.Npm, nil
			}
			if pSpec.Binary != "" {
				return &BinaryAdapter{}, pSpec.Binary, nil
			}
		}
	}

	// Try general adapters in priority order
	if spec.Brew != "" && commandExists("brew") {
		return &BrewAdapter{}, spec.Brew, nil
	}
	if spec.Npm != "" && commandExists("npm") {
		return &NpmAdapter{}, spec.Npm, nil
	}
	if spec.Pip != "" && commandExists("pip3") {
		return &PipAdapter{}, spec.Pip, nil
	}
	if spec.Cargo != "" && commandExists("cargo") {
		return &CargoAdapter{}, spec.Cargo, nil
	}

	// Last resort: binary download
	if spec.Platforms != nil {
		if pSpec, ok := spec.Platforms[platform]; ok && pSpec.Binary != "" {
			return &BinaryAdapter{}, pSpec.Binary, nil
		}
	}
	if spec.Source != "" && strings.HasPrefix(spec.Source, "https://") {
		return &BinaryAdapter{}, spec.Source, nil
	}

	return nil, "", fmt.Errorf("no suitable adapter found for platform %s", platform)
}

// InstallSpec mirrors the install section of ctx.yaml.
type InstallSpec struct {
	Source    string
	Brew      string
	Npm       string
	Pip       string
	Cargo     string
	Platforms map[string]PlatformSpec
}

// PlatformSpec is platform-specific install options.
type PlatformSpec struct {
	Source string
	Brew   string
	Npm    string
	Binary string
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runCommand executes a command and returns combined output.
func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}

// Verify checks if a binary is installed and runnable.
func Verify(binary, verifyCmd string) error {
	if verifyCmd != "" {
		parts := strings.Fields(verifyCmd)
		cmd := exec.Command(parts[0], parts[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("verify %q failed: %w", verifyCmd, err)
		}
		return nil
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("binary %q not found in PATH", binary)
	}
	return nil
}
