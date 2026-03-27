package auth

import (
	"github.com/getctx/ctx/internal/config"
)

// SaveToken stores the auth token in config.
func SaveToken(token, username string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = token
	cfg.Username = username
	return cfg.Save()
}

// ClearToken removes the auth token from config.
func ClearToken() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = ""
	cfg.Username = ""
	return cfg.Save()
}

// GetToken returns the current auth token.
func GetToken() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return cfg.Token, nil
}
