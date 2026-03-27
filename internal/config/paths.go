package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "ctx"

// Dir returns the ctx config directory (~/.ctx or XDG equivalent).
func Dir() string {
	if v := os.Getenv("CTX_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir(), "."+appName)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	return filepath.Join(homeDir(), "."+appName)
}

// DataDir returns the ctx data directory for installed packages.
func DataDir() string {
	if v := os.Getenv("CTX_DATA_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir(), "."+appName, "packages")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, appName, "packages")
	}
	return filepath.Join(homeDir(), "."+appName, "packages")
}

// CacheDir returns the ctx cache directory.
func CacheDir() string {
	if v := os.Getenv("CTX_CACHE_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir(), "Library", "Caches", appName)
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	return filepath.Join(homeDir(), ".cache", appName)
}

// ConfigFilePath returns the path to config.yaml.
func ConfigFilePath() string {
	return filepath.Join(Dir(), "config.yaml")
}

// LockFilePath returns the path to ctx.lock in the current directory.
func LockFilePath() string {
	return "ctx.lock"
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		// Fall back to HOME env var; if unavailable, panic as paths will be invalid
		if h = os.Getenv("HOME"); h == "" {
			panic("cannot determine home directory: " + err.Error())
		}
	}
	return h
}
