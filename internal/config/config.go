package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRegistry = "https://registry.getctx.org"
)

// Config represents the CLI configuration stored at ~/.ctx/config.yaml.
// Identity (username, auth token) is managed by the profile system (profiles.yaml + keychain).
type Config struct {
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`

	// Privacy settings
	UpdateCheck *bool  `yaml:"update_check,omitempty" json:"update_check,omitempty"`
	NetworkMode string `yaml:"network_mode,omitempty" json:"network_mode,omitempty"`
}

// IsUpdateCheckEnabled returns true if update checking is enabled (default: true).
func (c *Config) IsUpdateCheckEnabled() bool {
	return c.UpdateCheck == nil || *c.UpdateCheck
}

// IsOffline returns true if network mode is set to offline.
func (c *Config) IsOffline() bool {
	return c.NetworkMode == "offline"
}

// Load reads the config from disk. Returns default config if file doesn't exist.
func Load() (*Config, error) {
	c := &Config{
		Registry: DefaultRegistry,
	}
	path := ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Registry == "" {
		c.Registry = DefaultRegistry
	}
	return c, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	path := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// RegistryURL returns the configured registry URL.
func (c *Config) RegistryURL() string {
	if v := os.Getenv("CTX_REGISTRY"); v != "" {
		return v
	}
	return c.Registry
}

// WebURL derives the public web origin from the registry URL.
// "https://registry.getctx.org" → "https://getctx.org";
// URLs without a "registry." host prefix are returned as-is.
func (c *Config) WebURL() string {
	raw := c.RegistryURL()
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if after, ok := strings.CutPrefix(u.Host, "registry."); ok {
		u.Host = after
	}
	u.Path = ""
	return strings.TrimRight(u.String(), "/")
}

// PackageWebURL returns the web URL for a package detail page.
// SSOT for all user-facing package URLs in the CLI.
// Example: "https://getctx.org/package/@scope/name"
func (c *Config) PackageWebURL(fullName string) string {
	u, _ := url.JoinPath(c.WebURL(), "package", fullName)
	return u
}
