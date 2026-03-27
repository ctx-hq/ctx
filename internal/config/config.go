package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRegistry = "https://api.getctx.org"
)

// Config represents the CLI configuration stored at ~/.ctx/config.yaml.
type Config struct {
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
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

// IsLoggedIn returns true if the user has a token.
func (c *Config) IsLoggedIn() bool {
	return c.Token != ""
}

// RegistryURL returns the configured registry URL.
func (c *Config) RegistryURL() string {
	if v := os.Getenv("CTX_REGISTRY"); v != "" {
		return v
	}
	return c.Registry
}
